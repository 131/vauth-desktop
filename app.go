package main

import (
	"encoding/json"
	"os"
	"os/user"
	"strings"

	sshagentcrypt "github.com/131/ssh-agent-crypt"
	"github.com/131/vauth-desktop/internal/i18n"
	"github.com/131/vauth-desktop/internal/update"
	"github.com/131/vauth-desktop/internal/vault"
	"github.com/131/vauth-desktop/settings"
	vauthlogin "github.com/131/vauth-login"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// defaultLogin provides VAUTH_USER_LOGIN to .vauthrc macros such as
// `ssh_auth.role: $${env.VAUTH_USER_LOGIN}` when the desktop session lacks it:
// the operating system account name is used.
func defaultLogin() {
	if os.Getenv("VAUTH_USER_LOGIN") != "" {
		return
	}
	if current, err := user.Current(); err == nil {
		name := current.Username
		if _, after, found := strings.Cut(name, `\`); found {
			name = after
		}
		if name != "" {
			os.Setenv("VAUTH_USER_LOGIN", strings.ToLower(name))
		}
	}
}

// App exposes the UI methods; Open shows a URL in the default browser.
type App struct {
	Client  *vault.Client
	Open    func(string) error
	Agent   func() string
	Updater *update.Updater
	// Relaunch starts the updated executable and ends this instance.
	Relaunch func(path string) error
	// Reload renders the page again, in the current language.
	Reload   func()
	Settings *settings.Store
}

func NewApp(open func(string) error) *App {
	defaultLogin()
	store := settings.DefaultStore()
	current, _ := store.Load()
	i18n.Set(current.Language)
	return &App{Client: &vault.Client{New: vauthlogin.New}, Open: open, Agent: sshagentcrypt.DefaultAgentAddress, Updater: update.New(version, store, update.Authenticode), Settings: store}
}

func arg[T any](args []json.RawMessage, index int) (T, error) {
	var value T
	if index >= len(args) {
		return value, i18n.Errorf("err.argument")
	}
	err := json.Unmarshal(args[index], &value)
	return value, err
}

// Handle runs one UI method with JSON arguments.
func (a *App) Handle(method string, args []json.RawMessage) (any, error) {
	switch method {
	case "version":
		return version, nil
	case "connect":
		renew, err := arg[bool](args, 0)
		if err != nil {
			return nil, err
		}
		return a.Client.Connect(renew)
	case "read":
		return a.Client.Read()
	case "write":
		values, err := arg[map[string]any](args, 0)
		if err != nil {
			return nil, err
		}
		version, err := arg[int64](args, 1)
		if err != nil {
			return nil, err
		}
		return nil, a.Client.Write(values, version)
	case "variable_name":
		name, err := arg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return vault.VariableName(name)
	case "ssh_read":
		return a.Client.ReadSSH()
	case "ssh_publish":
		key, err := arg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return nil, a.Client.PublishSSH(key)
	case "agent_keys":
		return vault.AgentKeys(a.Agent())
	case "set_language":
		language, err := arg[string](args, 0)
		if err != nil {
			return nil, err
		}
		if !i18n.Supported(language) {
			return nil, i18n.Errorf("err.language", language)
		}
		if _, err := a.Settings.Update(func(s *settings.Settings) { s.Language = language }); err != nil {
			return nil, err
		}
		i18n.Set(language)
		if a.Reload != nil {
			a.Reload()
		}
		return nil, nil
	case "update_check":
		// update_check(true) is the manual check: manifest.json is fetched again.
		// A failed check still describes the settings, with its error.
		check := a.Updater.Check
		if refresh, _ := arg[bool](args, 0); refresh {
			check = a.Updater.CheckNow
		}
		release, err := check()
		if err != nil {
			release.Error = err.Error()
		}
		return release, nil
	case "update_enable":
		enable, err := arg[bool](args, 0)
		if err != nil {
			return nil, err
		}
		release, err := a.Updater.SetEnabled(enable)
		if err != nil {
			release.Error = err.Error()
		}
		return release, nil
	case "update_channel":
		channel, err := arg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.Updater.SetChannel(channel)
	case "update_install":
		tag, err := arg[string](args, 0)
		if err != nil {
			return nil, err
		}
		path, err := a.Updater.Install(tag)
		if err != nil {
			return nil, err
		}
		if a.Relaunch == nil {
			return nil, i18n.Errorf("err.restart")
		}
		return nil, a.Relaunch(path)
	case "update_open":
		// Development builds cannot update: open the release page of the channel.
		release, err := a.Updater.Check()
		if err != nil {
			return nil, err
		}
		if release.URL == "" {
			return nil, i18n.Errorf("err.update.none")
		}
		return nil, a.Open(release.URL)
	case "browse":
		link, err := a.Client.BrowseURL()
		if err != nil {
			return nil, err
		}
		return nil, a.Open(link)
	}
	return nil, i18n.Errorf("err.method", method)
}
