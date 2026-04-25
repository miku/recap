// Package cache stores and retrieves LLM summaries, grouped by input.
//
// All summaries of the same input text live under a single directory
// (named by SHA-256 of the input). Each file in that directory is one
// variant: a particular (model, style) generation. Filenames are
// human-readable slugs so that pointing a markdown renderer such as
// glow at the directory works directly:
//
//	glow -p ~/.cache/recap/<input-hash>
//
// On-disk format per file is YAML-style front matter (model, endpoint,
// style, timestamp, elapsed, sizes) followed by the response body.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Cache struct {
	root string
}

// Entry is one cached summary plus its metadata.
type Entry struct {
	Model       string
	Endpoint    string
	Style       string
	Created     time.Time
	Elapsed     time.Duration
	InputBytes  int
	OutputBytes int
	Body        string

	Path string // populated when read from disk
}

// New returns a cache rooted at root. If root is empty, the platform
// user-cache directory is used (e.g. $XDG_CACHE_HOME/recap on Linux).
func New(root string) (*Cache, error) {
	if root == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(base, "recap")
	}
	return &Cache{root: root}, nil
}

func (c *Cache) Root() string { return c.root }

// InputKey is the directory key for an input text.
func InputKey(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// Dir returns the directory holding all variants for inputKey.
func (c *Cache) Dir(inputKey string) string {
	return filepath.Join(c.root, inputKey)
}

// Slug normalizes a string for use as a filename component. Anything
// outside [A-Za-z0-9.-] becomes '-' so model names like "llama3.2:latest"
// map cleanly to "llama3.2-latest".
func Slug(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// Variants returns cached entries for inputKey that match (model, style).
// Used for ordinary cache hits.
func (c *Cache) Variants(inputKey, model, style string) ([]Entry, error) {
	pattern := filepath.Join(c.Dir(inputKey), Slug(model)+"__"+Slug(style)+"__*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	return loadAll(matches)
}

// All returns every cached entry for inputKey, newest first.
// Used for the "render every variant" view.
func (c *Cache) All(inputKey string) ([]Entry, error) {
	entries, err := os.ReadDir(c.Dir(inputKey))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		paths = append(paths, filepath.Join(c.Dir(inputKey), e.Name()))
	}
	out, err := loadAll(paths)
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Created.After(out[j].Created)
	})
	return out, nil
}

// PickRandom returns one entry at random. Caller must ensure non-empty.
func PickRandom(entries []Entry) Entry {
	return entries[rand.IntN(len(entries))]
}

// Put writes a new variant under inputKey. The filename is built from
// the model slug, style, and a short body hash so identical bodies
// dedupe naturally.
func (c *Cache) Put(inputKey string, e Entry) (string, error) {
	dir := c.Dir(inputKey)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	bodyHash := sha256.Sum256([]byte(e.Body))
	name := fmt.Sprintf("%s__%s__%s.md",
		Slug(e.Model), Slug(e.Style), hex.EncodeToString(bodyHash[:6]))
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, []byte(formatEntry(e)), 0o644)
}

func loadAll(paths []string) ([]Entry, error) {
	out := make([]Entry, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		e := parseEntry(string(raw))
		e.Path = p
		out = append(out, e)
	}
	return out, nil
}

func formatEntry(e Entry) string {
	var b strings.Builder
	b.WriteString("---\n")
	if e.Model != "" {
		fmt.Fprintf(&b, "model: %s\n", e.Model)
	}
	if e.Endpoint != "" {
		fmt.Fprintf(&b, "endpoint: %s\n", e.Endpoint)
	}
	if e.Style != "" {
		fmt.Fprintf(&b, "style: %s\n", e.Style)
	}
	if !e.Created.IsZero() {
		fmt.Fprintf(&b, "created: %s\n", e.Created.UTC().Format(time.RFC3339))
	}
	if e.Elapsed > 0 {
		fmt.Fprintf(&b, "elapsed_ms: %d\n", e.Elapsed.Milliseconds())
	}
	if e.InputBytes > 0 {
		fmt.Fprintf(&b, "input_bytes: %d\n", e.InputBytes)
	}
	if e.OutputBytes > 0 {
		fmt.Fprintf(&b, "output_bytes: %d\n", e.OutputBytes)
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimRight(e.Body, "\n"))
	b.WriteByte('\n')
	return b.String()
}

func parseEntry(s string) Entry {
	var e Entry
	if !strings.HasPrefix(s, "---\n") {
		e.Body = s
		return e
	}
	rest := s[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		// malformed: treat the whole thing as body
		e.Body = s
		return e
	}
	head := rest[:end]
	e.Body = strings.TrimLeft(rest[end+5:], "\n")
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "model":
			e.Model = v
		case "endpoint":
			e.Endpoint = v
		case "style":
			e.Style = v
		case "created":
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				e.Created = t
			}
		case "elapsed_ms":
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				e.Elapsed = time.Duration(n) * time.Millisecond
			}
		case "input_bytes":
			if n, err := strconv.Atoi(v); err == nil {
				e.InputBytes = n
			}
		case "output_bytes":
			if n, err := strconv.Atoi(v); err == nil {
				e.OutputBytes = n
			}
		}
	}
	return e
}
