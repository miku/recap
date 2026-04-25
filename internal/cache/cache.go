// Package cache stores LLM responses on disk, keyed by a hash of the
// inputs that determine the output (endpoint, model, rendered prompt).
//
// Because LLMs are probabilistic, each key maps to a directory and may
// hold multiple variants — one per generation. Reads pick one variant
// at random; writes append a new variant unless its content already
// exists (variants are themselves content-hashed).
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
)

type Cache struct {
	root string
}

// New returns a cache rooted at root. If root is empty, the user's
// platform cache directory is used (e.g. $XDG_CACHE_HOME/recap on Linux).
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

// Root returns the cache root directory.
func (c *Cache) Root() string { return c.root }

// Key derives a stable hash from the inputs that determine the output.
// Add new arguments here when new parameters (temperature, system prompt, ...)
// start influencing the response.
func Key(endpoint, model, prompt string) string {
	h := sha256.New()
	for _, s := range []string{endpoint, model, prompt} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Dir returns the directory holding all variants for key.
func (c *Cache) Dir(key string) string {
	return filepath.Join(c.root, key)
}

// List returns the variant filenames (not full paths) for key. An empty
// slice and a nil error means the key has no entries yet.
func (c *Cache) List(key string) ([]string, error) {
	entries, err := os.ReadDir(c.Dir(key))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// Get returns a random variant for key. ok is false if none exist.
func (c *Cache) Get(key string) (content string, ok bool, err error) {
	names, err := c.List(key)
	if err != nil {
		return "", false, err
	}
	if len(names) == 0 {
		return "", false, nil
	}
	pick := names[rand.IntN(len(names))]
	b, err := os.ReadFile(filepath.Join(c.Dir(key), pick))
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}

// Put records a new variant for key. The variant filename is derived
// from the content hash, so identical responses dedupe naturally.
func (c *Cache) Put(key, content string) error {
	dir := c.Dir(key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(content))
	name := hex.EncodeToString(sum[:8]) + ".txt"
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}
