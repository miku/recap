// Package prompts loads and renders summarization prompt templates.
//
// Prompt files live next to this source file as "*.md" and are embedded
// into the binary. Each file is a text/template; placeholders are filled
// from the [Data] struct at render time. To add a new style, drop a new
// file into this directory.
package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"text/template"
)

//go:embed *.md
var files embed.FS

// Data is the template context exposed to prompt files. Add fields here
// as more metadata (source URL, language, length hints, ...) is needed.
type Data struct {
	Text string
}

type prompt struct {
	name string
	tmpl *template.Template
}

var registry = map[string]*prompt{}

func init() {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		panic(fmt.Sprintf("prompts: read embed: %v", err))
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := fs.ReadFile(files, e.Name())
		if err != nil {
			panic(fmt.Sprintf("prompts: read %s: %v", e.Name(), err))
		}
		name := styleName(e.Name())
		t, err := template.New(name).Parse(string(raw))
		if err != nil {
			panic(fmt.Sprintf("prompts: parse %s: %v", e.Name(), err))
		}
		registry[name] = &prompt{name: name, tmpl: t}
	}
}

// styleName turns "0000-basic.md" into "basic" and "bullet-points.md" into
// "bullet-points". A numeric prefix followed by '-' is stripped so that
// files can be ordered on disk without affecting the public name.
func styleName(filename string) string {
	base := strings.TrimSuffix(filename, path.Ext(filename))
	if i := strings.IndexByte(base, '-'); i > 0 {
		prefix := base[:i]
		allDigits := true
		for _, c := range prefix {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return base[i+1:]
		}
	}
	return base
}

// List returns the available style names, sorted.
func List() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Render executes the named prompt template against data.
func Render(style string, data Data) (string, error) {
	p, ok := registry[style]
	if !ok {
		return "", fmt.Errorf("unknown style %q (available: %s)", style, strings.Join(List(), ", "))
	}
	var buf bytes.Buffer
	if err := p.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render %s: %w", style, err)
	}
	return buf.String(), nil
}
