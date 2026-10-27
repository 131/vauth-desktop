package vault

import (
	"net/url"
	"sort"
	"strings"
	"unicode"

	"github.com/131/vauth-desktop/v2/internal/i18n"
)

// Mount is an accessible KV engine, not a name-based allowlist.
type Mount struct {
	Path    string `json:"path"`
	Version int    `json:"version"`
}

type Folder struct {
	Keys []string `json:"keys"`
}

type Document struct {
	Values       map[string]any `json:"values"`
	Version      int64          `json:"version"`
	Exists       bool           `json:"exists"`
	Capabilities []string       `json:"capabilities"`
}

func logicalPath(value string, folder bool) (string, error) {
	if folder && value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\%?#") || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", i18n.Errorf("err.kv.path")
	}
	if folder {
		value = strings.TrimSuffix(value, "/")
	}
	parts := strings.Split(value, "/")
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", i18n.Errorf("err.kv.path")
		}
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/"), nil
}

func (c *Client) Mounts() ([]Mount, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return nil, err
	}
	result, err := c.call("GET", "sys/internal/ui/mounts", nil, false)
	if err != nil {
		return nil, err
	}
	data, ok := result["data"].(map[string]any)
	if !ok {
		return nil, i18n.Errorf("err.response")
	}
	engines, ok := data["secret"].(map[string]any)
	if !ok {
		return nil, i18n.Errorf("err.response")
	}
	c.mounts = map[string]Mount{}
	c.mountTypes = map[string]string{}
	list := []Mount{}
	for path, raw := range engines {
		engine, ok := raw.(map[string]any)
		if !ok {
			return nil, i18n.Errorf("err.response")
		}
		if !strings.HasSuffix(path, "/") {
			return nil, i18n.Errorf("err.kv.path")
		}
		if _, err := logicalPath(strings.TrimSuffix(path, "/"), false); err != nil {
			return nil, err
		}
		kind, _ := engine["type"].(string)
		c.mountTypes[path] = kind
		if kind != "kv" {
			continue
		}
		version := 1
		if options, ok := engine["options"].(map[string]any); ok {
			if value, ok := options["version"].(string); ok && value != "" {
				switch value {
				case "1":
					version = 1
				case "2":
					version = 2
				default:
					version = 0
				}
			}
		}
		mount := Mount{Path: path, Version: version}
		c.mounts[path] = mount
		list = append(list, mount)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
	return list, nil
}

func (c *Client) kvRoute(mount, path string, folder bool) (string, Mount, error) {
	m, ok := c.mounts[mount]
	if !ok {
		return "", m, i18n.Errorf("err.kv.mount")
	}
	if m.Version != 1 && m.Version != 2 {
		return "", m, i18n.Errorf("err.kv.version")
	}
	escaped, err := logicalPath(path, folder)
	if err != nil {
		return "", m, err
	}
	root, err := logicalPath(strings.TrimSuffix(m.Path, "/"), false)
	if err != nil {
		return "", m, err
	}
	route := root + "/"
	if m.Version == 2 {
		if folder {
			route += "metadata/"
		} else {
			route += "data/"
		}
	}
	route += escaped
	if folder && escaped != "" {
		route += "/"
	}
	decoded, err := url.PathUnescape(route)
	if err != nil {
		return "", m, i18n.Errorf("err.kv.path")
	}
	for other := range c.mountTypes {
		if other != m.Path && len(other) > len(m.Path) && strings.HasPrefix(decoded, other) {
			return "", m, i18n.Errorf("err.kv.mount")
		}
	}
	return route, m, nil
}

func allowed(caps []string, action string) bool {
	for _, cap := range caps {
		if cap == "deny" {
			return false
		}
	}
	for _, cap := range caps {
		if cap == action || cap == "root" {
			return true
		}
	}
	return false
}

// capabilities checks the actual API route, never extrapolating root rights.
func (c *Client) capabilities(route string) ([]string, error) {
	path, err := url.PathUnescape(route)
	if err != nil {
		return nil, err
	}
	result, err := c.call("POST", "sys/capabilities-self", map[string]any{"paths": []string{path}}, false)
	if err != nil {
		return nil, err
	}
	if data, ok := result["data"].(map[string]any); ok {
		result = data
	}
	raw, ok := result[path].([]any)
	if !ok {
		raw, ok = result["capabilities"].([]any)
	}
	if !ok {
		return nil, i18n.Errorf("err.response")
	}
	caps := []string{}
	for _, item := range raw {
		cap, ok := item.(string)
		if !ok {
			return nil, i18n.Errorf("err.response")
		}
		caps = append(caps, cap)
	}
	return caps, nil
}

func (c *Client) ListKV(mount, path string) (*Folder, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return nil, err
	}
	route, _, err := c.kvRoute(mount, path, true)
	if err != nil {
		return nil, err
	}
	caps, err := c.capabilities(route)
	if err != nil {
		return nil, err
	}
	if !allowed(caps, "list") {
		return nil, &VaultError{403}
	}
	result, err := c.call("LIST", route, nil, true)
	if err != nil {
		return nil, err
	}
	folder := &Folder{Keys: []string{}}
	if result != nil {
		data, ok := result["data"].(map[string]any)
		if !ok {
			return nil, i18n.Errorf("err.response")
		}
		raw, ok := data["keys"].([]any)
		if !ok {
			return nil, i18n.Errorf("err.response")
		}
		for _, key := range raw {
			name, ok := key.(string)
			if !ok {
				return nil, i18n.Errorf("err.response")
			}
			// Returned names must be direct children; nested paths are navigated lazily.
			if strings.Contains(strings.TrimSuffix(name, "/"), "/") {
				return nil, i18n.Errorf("err.kv.path")
			}
			if _, err := logicalPath(name, strings.HasSuffix(name, "/")); err != nil {
				return nil, err
			}
			folder.Keys = append(folder.Keys, name)
		}
	}
	sort.Strings(folder.Keys)
	return folder, nil
}

func (c *Client) ReadKV(mount, path string) (*Document, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return nil, err
	}
	route, m, err := c.kvRoute(mount, path, false)
	if err != nil {
		return nil, err
	}
	caps, err := c.capabilities(route)
	if err != nil {
		return nil, err
	}
	doc := &Document{Values: map[string]any{}, Capabilities: caps}
	if !allowed(caps, "read") {
		return nil, &VaultError{403}
	}
	result, err := c.call("GET", route, nil, true)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return doc, nil
	}
	data, ok := result["data"].(map[string]any)
	if !ok {
		return nil, i18n.Errorf("err.response")
	}
	doc.Exists = true
	if m.Version == 1 {
		doc.Values = data
		return doc, nil
	}
	metadata, ok := data["metadata"].(map[string]any)
	if !ok {
		return nil, i18n.Errorf("err.response")
	}
	version, ok := metadata["version"].(float64)
	if !ok || version < 1 {
		return nil, i18n.Errorf("err.response")
	}
	doc.Version = int64(version)
	if deleted, _ := metadata["deletion_time"].(string); deleted != "" {
		doc.Exists = false
		return doc, nil
	}
	if destroyed, _ := metadata["destroyed"].(bool); destroyed {
		doc.Exists = false
		return doc, nil
	}
	values, ok := data["data"].(map[string]any)
	if !ok {
		return nil, i18n.Errorf("err.response")
	}
	doc.Values = values
	return doc, nil
}

func (c *Client) KVCapabilities(mount, path string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return nil, err
	}
	route, _, err := c.kvRoute(mount, path, false)
	if err != nil {
		return nil, err
	}
	return c.capabilities(route)
}

func (c *Client) WriteKV(mount, path string, values map[string]any, version int64, create bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return err
	}
	route, m, err := c.kvRoute(mount, path, false)
	if err != nil {
		return err
	}
	if values == nil || version < 0 || (create && version != 0) || (!create && m.Version == 2 && version < 1) {
		return i18n.Errorf("err.argument")
	}
	caps, err := c.capabilities(route)
	if err != nil {
		return err
	}
	action := "update"
	if create {
		action = "create"
	}
	if !allowed(caps, action) {
		return &VaultError{403}
	}
	var payload any = values
	if m.Version == 2 {
		payload = map[string]any{"data": values, "options": map[string]any{"cas": version}}
	}
	_, err = c.call("POST", route, payload, false)
	return err
}

func (c *Client) DeleteKV(mount, path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return err
	}
	route, _, err := c.kvRoute(mount, path, false)
	if err != nil {
		return err
	}
	caps, err := c.capabilities(route)
	if err != nil {
		return err
	}
	if !allowed(caps, "delete") {
		return &VaultError{403}
	}
	_, err = c.call("DELETE", route, nil, false)
	return err
}
