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
	if strings.Contains(page, `href="generated/app.css"`) || strings.Contains(page, `src="generated/app.js"`) || !strings.Contains(page, "window.vauthDone") {
		t.Fatal("assets not inlined")
	}
	if !strings.Contains(page, `default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'`) {
		t.Fatal("native document CSP changed")
	}
	if regexp.MustCompile(`<script[^>]+src=|<link[^>]+rel="stylesheet"`).MatchString(page) {
		t.Fatal("standalone document references external assets")
	}
	if !strings.Contains(page, `<html lang="fr">`) || !strings.Contains(page, `"connect":"Se connecter avec mon agent SSH"`) {
		t.Fatal("french page")
	}
}

func TestFrontendNoticeEmbedded(t *testing.T) {
	notice, err := os.ReadFile("FRONTEND-NOTICE.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(notice) == 0 || bytes.Contains(notice, []byte("*/")) || bytes.Contains(bytes.ToLower(notice), []byte("</script")) {
		t.Fatal("frontend notice is empty or unsafe as an inline script comment")
	}
	bundle, err := assets.ReadFile("generated/app.js")
	if err != nil {
		t.Fatal(err)
	}
	comment := "/*!\n" + string(notice) + "*/"
	if !strings.HasPrefix(string(bundle), comment) {
		t.Fatal("compiled frontend bundle must carry the complete notice")
	}
	for _, language := range i18n.Languages {
		if !strings.Contains(Page(language), comment) {
			t.Fatalf("%s: standalone document lacks frontend notices", language)
		}
	}
}

func TestEverySnippetKeyTranslated(t *testing.T) {
	source, err := os.ReadFile("../src/App.svelte")
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range i18n.Languages {
		for _, match := range regexp.MustCompile(`\bt\('([^']+)'`).FindAllStringSubmatch(string(source), -1) {
			if _, ok := i18n.Messages(language)[match[1]]; !ok {
				t.Errorf("%s missing snippet key %s", language, match[1])
			}
		}
		page := Page(language)
		if leftover := regexp.MustCompile(`\{\{[^}]*\}\}`).FindString(page); leftover != "" {
			t.Fatalf("%s: %s left", language, leftover)
		}
	}
	if page := Page("en-us"); !strings.Contains(page, `<html lang="en">`) || !strings.Contains(page, `"connect":"Sign in with my SSH agent"`) || !strings.Contains(page, `"language":"en-us"`) {
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
			// Dependencies and Git metadata are not maintained project source.
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
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
	script, _ := os.ReadFile("app.js")
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
	english, french, chinese := i18n.Messages("en-us"), i18n.Messages("fr-fr"), i18n.Messages("zh-cn")
	for key, value := range french {
		if strings.Contains(strings.ToLower(value), "jeton") {
			t.Errorf("%s: fr-fr must retain the term token", key)
		}
	}
	for _, key := range []string{"tab.wrap", "tab.unwrap", "wrap.submit", "wrap.unwrap", "wrap.token", "unwrap.token", "wrap.data", "wrap.result", "wrap.ttl", "wrap.copy.data"} {
		if english[key] == french[key] {
			t.Errorf("%s: French still uses English wording", key)
		}
		if !strings.Contains(spec, "| "+english[key]+" | "+french[key]+" | "+chinese[key]+" |") {
			t.Errorf("%s: terminology table and catalog differ", key)
		}
	}
}
