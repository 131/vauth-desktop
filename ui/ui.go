// Package ui embeds the built Svelte document and its native controller.
// Go supplies the language catalog before the shared UI mounts.
package ui

import (
	"embed"
	"encoding/json"
	"html"
	"regexp"
	"strings"

	"github.com/131/vauth-desktop/v2/internal/i18n"
)

var placeholder = regexp.MustCompile(`\{\{([a-z0-9.]+)\}\}`)

//go:embed index.html generated/app.css generated/app.js
var assets embed.FS

// Page returns the UI as a single document in language, with style and
// script inlined.
func Page(language string) string {
	read := func(name string) string {
		body, err := assets.ReadFile(name)
		if err != nil {
			panic(err)
		}
		return string(body)
	}
	messages := i18n.Messages(language)
	page := placeholder.ReplaceAllStringFunc(read("index.html"), func(match string) string {
		return html.EscapeString(messages[match[2:len(match)-2]])
	})
	state, _ := json.Marshal(map[string]any{"language": i18n.Normalize(language), "languages": i18n.Languages, "messages": messages})
	page = strings.Replace(page, `<link rel="stylesheet" href="generated/app.css">`, "<style>"+read("generated/app.css")+"</style>", 1)
	page = strings.Replace(page, `<script src="generated/app.js"></script>`, "<script>window.I18N="+string(state)+";</script><script>"+read("generated/app.js")+"</script>", 1)
	return page
}
