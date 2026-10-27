package vault

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	sshagentcrypt "github.com/131/ssh-agent-crypt"
	"github.com/131/vauth-desktop/v2/internal/i18n"
	vauthlogin "github.com/131/vauth-login"
)

// Personal document shared with `vauth set` and venv.
const databaseName = ".vauth_database"

// Identity metadata is read-only: it comes from the Vault entity.
var fixedVars = []string{"VAUTH_USER_LOGIN", "VAUTH_USER_NAME", "VAUTH_USER_MAIL"}

// VaultError carries the HTTP status with a user-facing message.
type VaultError struct{ Status int }

func (e *VaultError) Error() string {
	switch e.Status {
	case 403:
		return i18n.T("err.vault.403")
	case 400:
		return i18n.T("err.vault.400")
	}
	return i18n.T("err.vault", e.Status)
}

// Client holds the user's Vault session, established by vauth-login.
type Client struct {
	// New builds a fresh vauth-login client, so each refresh goes through
	// its token cache and renewal logic.
	New func() (*vauthlogin.Vauth, error)
	Now func() time.Time
	// OIDC state and KV routes stay in Go, never in the page.
	pending    *oidcAttempt
	listenOIDC func() (net.Listener, error)
	ssoTimeout time.Duration

	mu         sync.Mutex
	vauth      *vauthlogin.Vauth
	token      string
	refreshAt  time.Time
	login      string
	profile    map[string]string
	sso        bool
	expiresAt  time.Time
	renewable  bool
	mounts     map[string]Mount
	mountTypes map[string]string
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// connect obtains a token and schedules its refresh in the last third of its lease.
func (c *Client) connect(renew bool) error {
	v, err := c.New()
	if err != nil {
		return err
	}
	v.Stderr = io.Discard
	token, err := v.Connect(renew)
	if err != nil {
		return agentError(err)
	}
	if token == "" {
		return i18n.Errorf("err.auth")
	}
	lookup, err := v.LookupSelf(token)
	if err != nil {
		return vaultError(err)
	}
	c.vauth, c.token = v, token
	c.refreshAt = time.Time{}
	if lookup.TTL > 0 {
		margin := float64(lookup.LeaseDuration) / 3
		if margin <= 0 || margin > lookup.TTL {
			margin = lookup.TTL / 3
		}
		c.refreshAt = c.now().Add(time.Duration((lookup.TTL - margin) * float64(time.Second)))
	}
	return nil
}

// agentError turns vauth-login's raw agent failures (pipe paths, OS errors)
// into short translated messages; the cause stays wrapped for errors.Is.
func agentError(err error) error {
	message := err.Error()
	switch {
	case strings.Contains(message, "Could not reach ssh-agent"):
		return &friendlyError{i18n.T("err.agent.unreachable"), err}
	case strings.Contains(message, "Unable to list ssh-agent keys"):
		return &friendlyError{i18n.T("err.agent.list"), err}
	}
	return err
}

// friendlyError shows a translated message and keeps the cause for errors.Is.
type friendlyError struct {
	message string
	cause   error
}

func (e *friendlyError) Error() string { return e.message }
func (e *friendlyError) Unwrap() error { return e.cause }

func vaultError(err error) error {
	var failure *vauthlogin.HTTPError
	if errors.As(err, &failure) {
		return &VaultError{failure.Status}
	}
	return err
}

// call sends a Vault request, refreshing the session when due and logging in
// again once when the token is no longer valid. A nil result means 404 when optional.
func (c *Client) call(method, path string, payload any, optional bool) (map[string]any, error) {
	if c.sso {
		if !c.expiresAt.IsZero() && !c.now().Before(c.expiresAt) {
			c.clear()
			return nil, ErrSessionExpired
		}
		if c.renewable && !c.refreshAt.IsZero() && !c.now().Before(c.refreshAt) {
			if err := c.renewSSO(); err != nil {
				return nil, err
			}
		}
	}
	if c.vauth == nil {
		if err := c.connect(false); err != nil {
			return nil, err
		}
	} else if !c.sso && !c.refreshAt.IsZero() && !c.now().Before(c.refreshAt) {
		if err := c.connect(false); err != nil {
			return nil, err
		}
	}
	for attempt := 0; ; attempt++ {
		status, body, err := c.vauth.Request(method, path, c.token, payload, 0)
		if err != nil {
			return nil, err
		}
		if status == 403 && attempt == 0 && !c.valid() {
			if c.sso {
				c.clear()
				return nil, ErrSessionExpired
			}
			if err := c.connect(true); err != nil {
				return nil, err
			}
			continue
		}
		if optional && status == 404 {
			return nil, nil
		}
		if status < 200 || status >= 300 {
			return nil, &VaultError{status}
		}
		result := map[string]any{}
		if status != 204 && len(body) != 0 {
			if err := json.Unmarshal(body, &result); err != nil {
				return nil, i18n.Errorf("err.response")
			}
		}
		return result, nil
	}
}

func (c *Client) valid() bool {
	status, _, err := c.vauth.Request("GET", "auth/token/lookup-self", c.token, nil, 0)
	return err == nil && status == 200
}

// Session describes the connected user.
type Session struct {
	Login     string            `json:"login"`
	VaultAddr string            `json:"vault_addr"`
	Profile   map[string]string `json:"profile"`
	FixedVars []string          `json:"fixed_vars"`
	AuthMode  string            `json:"auth_mode"`
}

// Connect authenticates (renew forces a new login) and loads the Vault entity profile.
func (c *Client) Connect(renew bool) (*Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancelSSO()
	if renew || c.vauth == nil {
		next := &Client{New: c.New, Now: c.Now}
		if err := next.connect(renew); err != nil {
			return nil, err
		}
		session, err := next.loadProfile()
		if err != nil {
			return nil, err
		}
		c.adopt(next)
		return session, nil
	}
	return c.loadProfile()
}

func (c *Client) loadProfile() (*Session, error) {
	lookup, err := c.call("GET", "auth/token/lookup-self", nil, false)
	if err != nil {
		return nil, err
	}
	data, _ := lookup["data"].(map[string]any)
	if policies, ok := data["policies"].([]any); ok {
		for _, policy := range policies {
			if policy == "root" {
				return nil, i18n.Errorf("err.root")
			}
		}
	}
	entityID, _ := data["entity_id"].(string)
	if entityID == "" {
		return nil, i18n.Errorf("err.entity")
	}
	entity, err := c.call("GET", "identity/entity/id/"+url.PathEscape(entityID), nil, false)
	if err != nil {
		return nil, err
	}
	profile := map[string]string{}
	if data, ok := entity["data"].(map[string]any); ok {
		if metadata, ok := data["metadata"].(map[string]any); ok {
			for key, value := range metadata {
				profile[key] = fmt.Sprint(value)
			}
		}
	}
	if profile["VAUTH_USER_LOGIN"] == "" {
		return nil, i18n.Errorf("err.profile")
	}
	c.login = profile["VAUTH_USER_LOGIN"]
	c.profile = profile
	mode := "ssh"
	if c.sso {
		mode = "sso"
	}
	return &Session{Login: c.login, VaultAddr: c.vauth.VAULT_ADDR(), Profile: profile, FixedVars: fixedVars, AuthMode: mode}, nil
}

func (c *Client) connected() error {
	if c.login == "" {
		return i18n.Errorf("err.session")
	}
	return nil
}

func (c *Client) databasePath() string {
	return "private/" + url.PathEscape(c.login) + "/data/" + url.PathEscape(databaseName)
}

// Database is the personal dictionary with its KV v2 version for compare-and-set.
type Database struct {
	Values  map[string]any `json:"values"`
	Version int64          `json:"version"`
}

func (c *Client) Read() (*Database, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return nil, err
	}
	record, err := c.call("GET", c.databasePath(), nil, true)
	if err != nil {
		return nil, err
	}
	database := &Database{Values: map[string]any{}}
	if data, ok := record["data"].(map[string]any); ok {
		if values, ok := data["data"].(map[string]any); ok {
			database.Values = values
		}
		if metadata, ok := data["metadata"].(map[string]any); ok {
			if version, ok := metadata["version"].(float64); ok {
				database.Version = int64(version)
			}
		}
	}
	return database, nil
}

// Write replaces the dictionary unless another writer changed it since version.
func (c *Client) Write(values map[string]any, version int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return err
	}
	for key := range values {
		if _, reserved := c.profile[key]; reserved {
			return i18n.Errorf("err.reserved")
		}
		if normalized, err := VariableName(key); err != nil {
			return err
		} else if normalized != key {
			return i18n.Errorf("err.name")
		}
	}
	_, err := c.call("POST", c.databasePath(), map[string]any{"data": values, "options": map[string]any{"cas": version}}, false)
	return err
}

var variablePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// VariableName normalizes a key like `vauth set` and refuses identity names.
func VariableName(value string) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(value))
	if !variablePattern.MatchString(key) {
		return "", i18n.Errorf("err.name")
	}
	for _, fixed := range fixedVars {
		if key == fixed {
			return "", i18n.Errorf("err.reserved")
		}
	}
	return key, nil
}

func (c *Client) sshPath() string {
	path := "ssh"
	if c.vauth != nil {
		if auth, ok := c.vauth.RC()["ssh_auth"].(map[string]any); ok {
			if configured, ok := auth["path"].(string); ok && configured != "" {
				path = strings.Trim(configured, "/")
			}
		}
	}
	return "auth/" + path + "/role/" + url.PathEscape(c.login)
}

// ReadSSH returns the public keys authorized on the user's SSH role.
func (c *Client) ReadSSH() ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return nil, err
	}
	role, err := c.call("GET", c.sshPath(), nil, false)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	if data, ok := role["data"].(map[string]any); ok {
		if list, ok := data["public_keys"].([]any); ok {
			for _, key := range list {
				if key, ok := key.(string); ok {
					keys = append(keys, key)
				}
			}
		}
	}
	return keys, nil
}

var publicKeyPattern = regexp.MustCompile(`^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp(?:256|384|521)) [A-Za-z0-9+/]+={0,2}(?: [^\r\n]*)?$`)

// ValidateSSH accepts a single public key line, never private key material.
func ValidateSSH(value string) (string, error) {
	key := strings.TrimSpace(value)
	if strings.Contains(key, "PRIVATE KEY") || !publicKeyPattern.MatchString(key) {
		return "", i18n.Errorf("err.public.key")
	}
	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key)); err != nil {
		return "", i18n.Errorf("err.public.key")
	}
	return key, nil
}

// PublishSSH replaces the role's public keys with key.
func (c *Client) PublishSSH(value string) error {
	key, err := ValidateSSH(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return err
	}
	caps, err := c.capabilities(c.sshPath())
	if err != nil {
		return err
	}
	if !allowed(caps, "update") && !allowed(caps, "create") {
		return &VaultError{403}
	}
	_, err = c.call("POST", c.sshPath(), map[string]any{"public_keys": []string{key}}, false)
	return err
}

// AgentKey is a public key offered by the local ssh-agent or Pageant.
type AgentKey struct {
	Key         string `json:"key"`
	Comment     string `json:"comment"`
	Fingerprint string `json:"fingerprint"`
}

// AgentKeys lists the agent's public keys, in authorized_keys format.
func AgentKeys(address string) ([]AgentKey, error) {
	if address == "" {
		return nil, i18n.Errorf("err.agent.none")
	}
	conn, err := sshagentcrypt.DialAgent(address)
	if err != nil {
		return nil, i18n.Errorf("err.agent.unreachable")
	}
	defer conn.Close()
	keys, err := agent.NewClient(conn).List()
	if err != nil {
		return nil, i18n.Errorf("err.agent.list")
	}
	result := []AgentKey{}
	for _, key := range keys {
		line := key.Type() + " " + base64.StdEncoding.EncodeToString(key.Marshal())
		if key.Comment != "" {
			line += " " + key.Comment
		}
		if _, err := ValidateSSH(line); err != nil {
			continue
		}
		result = append(result, AgentKey{Key: line, Comment: key.Comment, Fingerprint: ssh.FingerprintSHA256(key)})
	}
	return result, nil
}
