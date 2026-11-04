package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsAndPartialFile(t *testing.T) {
	store := &Store{Path: filepath.Join(t.TempDir(), "settings.yml")}
	if settings, err := store.Load(); err != nil || settings != Defaults() {
		t.Fatalf("missing file: %+v %v", settings, err)
	}
	os.WriteFile(store.Path, []byte("update:\n  enable: false\n  upstream: https://github.com/me/fork/\n"), 0o600)
	settings, err := store.Load()
	want := Update{Enable: false, Upstream: "https://github.com/me/fork", Channel: DefaultChannel}
	if err != nil || settings.Update != want {
		t.Fatalf("partial file: %+v %v", settings, err)
	}
	for language, want := range map[string]string{"en-us": "en-us", "EN-US": "en-us", "de-de": "fr-fr", "": "fr-fr"} {
		os.WriteFile(store.Path, []byte("language: "+language+"\n"), 0o600)
		if settings, err := store.Load(); err != nil || settings.Language != want {
			t.Fatalf("language %q: %+v %v", language, settings, err)
		}
	}
}

func TestUpdateWritesYAML(t *testing.T) {
	store := &Store{Path: filepath.Join(t.TempDir(), "vauth-desktop", "settings.yml")}
	_, err := store.Update(func(s *Settings) { s.Update.Channel = "beta" })
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(store.Path)
	for _, line := range []string{"enable: true", "upstream: " + DefaultUpstream, "channel: beta"} {
		if !strings.Contains(string(body), line) {
			t.Fatalf("missing %q", line)
		}
	}
}

func TestLegacyCacheIsIgnored(t *testing.T) {
	store := &Store{Path: filepath.Join(t.TempDir(), "settings.yml")}
	os.WriteFile(store.Path, []byte("language: zh-cn\nremote_channels:\n  ttl: 86400\n  list:\n    production: v2.2.1\n"), 0600)
	if current, err := store.Load(); err != nil || current.Language != "zh-cn" {
		t.Fatalf("legacy settings: %+v %v", current, err)
	}
	if _, err := store.Update(func(*Settings) {}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(store.Path)
	if strings.Contains(string(body), "remote_channels") {
		t.Fatal("legacy cache retained")
	}
}

func TestUnreadableFileKept(t *testing.T) {
	store := &Store{Path: filepath.Join(t.TempDir(), "settings.yml")}
	os.WriteFile(store.Path, []byte("update: [\n"), 0o600)
	if _, err := store.Load(); err == nil {
		t.Fatal("invalid YAML accepted")
	}
	if _, err := store.Update(func(s *Settings) { s.Update.Channel = "beta" }); err == nil {
		t.Fatal("invalid file overwritten")
	}
	if body, _ := os.ReadFile(store.Path); string(body) != "update: [\n" {
		t.Fatal("invalid file modified")
	}
}
