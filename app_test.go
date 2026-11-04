package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/131/vauth-desktop/v2/internal/i18n"
	"github.com/131/vauth-desktop/v2/internal/vault"
	"github.com/131/vauth-desktop/v2/settings"
)

func TestSetLanguage(t *testing.T) {
	defer i18n.Set(i18n.Default)
	reloaded := 0
	app := &App{Settings: &settings.Store{Path: filepath.Join(t.TempDir(), "settings.yml")}, Reload: func() { reloaded++ }}
	if _, err := app.Handle("set_language", []json.RawMessage{json.RawMessage(`"de-de"`)}); err == nil {
		t.Fatal("unknown language accepted")
	}
	if _, err := app.Handle("set_language", []json.RawMessage{json.RawMessage(`"en-us"`)}); err != nil {
		t.Fatal(err)
	}
	saved, _ := app.Settings.Load()
	if saved.Language != "en-us" || i18n.Current() != "en-us" || reloaded != 1 {
		t.Fatalf("%+v %s %d", saved, i18n.Current(), reloaded)
	}
	if _, err := app.Handle("nope", nil); err == nil || err.Error() != "unknown method: nope" {
		t.Fatal(err)
	}
}

func TestKVArgumentsAndRemovedMethod(t *testing.T) {
	app := &App{Client: &vault.Client{}}
	for _, method := range []string{"kv_list", "kv_read", "kv_capabilities", "kv_write", "kv_delete"} {
		if _, err := app.Handle(method, nil); err == nil {
			t.Error("accepted missing arguments", method)
		}
		if _, err := app.Handle(method, []json.RawMessage{json.RawMessage(`42`)}); err == nil {
			t.Error("accepted wrong mount type", method)
		}
	}
	if _, err := app.Handle("browse", nil); err == nil {
		t.Fatal("removed Vault browser method still works")
	}
	if result, err := app.Handle("session", nil); err != nil || result.(*vault.Session) != nil {
		t.Fatal("signed-out session unexpectedly authenticated")
	}
}

func TestWrappingArguments(t *testing.T) {
	app := &App{Client: &vault.Client{}}
	for _, args := range [][]json.RawMessage{
		nil, {json.RawMessage(`[]`)}, {json.RawMessage(`null`)},
		{json.RawMessage(`{}`)}, {json.RawMessage(`42`)}, {json.RawMessage(`"  "`), json.RawMessage(`900`)},
		{json.RawMessage(`"message"`)}, {json.RawMessage(`"message"`), json.RawMessage(`"900"`)},
		{json.RawMessage(`"message"`), json.RawMessage(`0`)}, {json.RawMessage(`"message"`), json.RawMessage(`300`)},
	} {
		if _, err := app.Handle("wrap", args); err == nil {
			t.Fatal("invalid wrap accepted")
		}
	}
	for _, args := range [][]json.RawMessage{nil, {json.RawMessage(`42`)}, {json.RawMessage(`"  "`)}} {
		if _, err := app.Handle("unwrap", args); err == nil {
			t.Fatal("invalid unwrap accepted")
		}
	}
}

func TestSetChineseLanguage(t *testing.T) {
	defer i18n.Set(i18n.Default)
	reloads := 0
	app := &App{Settings: &settings.Store{Path: filepath.Join(t.TempDir(), "settings.yml")}, Reload: func() { reloads++ }}
	if _, err := app.Handle("set_language", []json.RawMessage{json.RawMessage(`"zh-cn"`)}); err != nil {
		t.Fatal(err)
	}
	saved, err := app.Settings.Load()
	if err != nil || saved.Language != "zh-cn" || i18n.Current() != "zh-cn" || reloads != 1 {
		t.Fatalf("%+v %v %d", saved, err, reloads)
	}
}
