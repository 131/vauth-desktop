package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/131/vauth-desktop/v2/internal/i18n"
)

func TestPage(t *testing.T) {
	page := Page("fr-fr")
	if strings.Contains(page, `href="style.css"`) || strings.Contains(page, `src="app.js"`) || !strings.Contains(page, "window.vauthDone") {
		t.Fatal("assets not inlined")
	}
	if !strings.Contains(page, `<html lang="fr">`) || !strings.Contains(page, "Se connecter avec mon agent SSH") {
		t.Fatal("french page")
	}
}

func TestEveryPlaceholderTranslated(t *testing.T) {
	html, _ := assets.ReadFile("index.html")
	for _, language := range []string{"fr-fr", "en-us"} {
		for _, match := range regexp.MustCompile(`\{\{([^}]+)\}\}`).FindAllStringSubmatch(string(html), -1) {
			if _, ok := i18n.Messages(language)[match[1]]; !ok {
				t.Errorf("%s missing placeholder %s", language, match[1])
			}
		}
		page := Page(language)
		if leftover := regexp.MustCompile(`\{\{[^}]*\}\}`).FindString(page); leftover != "" {
			t.Fatalf("%s: %s left", language, leftover)
		}
	}
	if page := Page("en-us"); !strings.Contains(page, `<html lang="en">`) || !strings.Contains(page, "Sign in with my SSH agent") || !strings.Contains(page, `"language":"en-us"`) {
		t.Fatal("english page")
	}
}

func TestNeutralProjectContents(t *testing.T) {
	// Numeric spelling keeps the terms themselves out of repository contents.
	terms := [][]byte{{109, 101, 100, 116, 101, 99, 104}, {102, 108, 101, 117, 114, 101, 110, 116}}
	err := filepath.WalkDir("..", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, term := range terms {
			if bytes.Contains(bytes.ToLower(data), term) {
				t.Errorf("Internal identifier in %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEveryScriptKeyExists(t *testing.T) {
	page := Page("en-us")
	script, _ := assets.ReadFile("app.js")
	for _, match := range regexp.MustCompile(`\bt\('([^']+)'`).FindAllStringSubmatch(string(script), -1) {
		if !strings.Contains(page, `"`+match[1]+`":`) {
			t.Errorf("app.js uses unknown message %s", match[1])
		}
	}
}

func TestTokenWrapLanguageAndTerminology(t *testing.T) {
	data, err := os.ReadFile("../specs/TOKEN_WRAP.md")
	if err != nil {
		t.Fatal(err)
	}
	spec := string(data)
	if !strings.Contains(spec, "## Language & terminology") {
		t.Fatal("terminology section missing")
	}
	english, french := i18n.Messages("en-us"), i18n.Messages("fr-fr")
	for _, key := range []string{"tab.wrap", "tab.unwrap", "wrap.submit", "wrap.unwrap", "wrap.token", "unwrap.token", "wrap.data", "wrap.result", "wrap.ttl", "wrap.copy.data"} {
		if english[key] == french[key] {
			t.Errorf("%s: French still uses English wording", key)
		}
		if !strings.Contains(spec, "| "+english[key]+" | "+french[key]+" |") {
			t.Errorf("%s: terminology table and catalog differ", key)
		}
	}
}
