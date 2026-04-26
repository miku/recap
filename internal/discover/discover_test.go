package discover

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://localhost:11434", "http://localhost:11434"},
		{"http://localhost:11434/", "http://localhost:11434"},
		{"https://api.example.com", "https://api.example.com"},
		{"localhost:11434", "http://localhost:11434"},
		{"127.0.0.1:11434", "http://127.0.0.1:11434"},
		{"http://host:11434/some/path", "http://host:11434"},
	}
	for _, c := range cases {
		if got := normalizeHost(c.in); got != c.want {
			t.Errorf("normalizeHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsEmbedding(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"nomic-embed-text", true},
		{"bge-m3", true},
		{"llama-embed", true},
		{"mxbai-embed-large", true},
		{"llama3.2:latest", false},
		{"gpt-4o", false},
		{"qwen2.5-coder:7b", false},
	}
	for _, c := range cases {
		if got := isEmbedding(c.name); got != c.want {
			t.Errorf("isEmbedding(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPickOllamaModelPrefersNewestNonEmbedding(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}
		// Embedding model is newest — should be deprioritized.
		fmt.Fprint(w, `{"models":[
			{"name":"old:latest","modified_at":"2025-01-01T00:00:00Z"},
			{"name":"new:latest","modified_at":"2026-04-01T00:00:00Z"},
			{"name":"nomic-embed-text:latest","modified_at":"2026-04-26T00:00:00Z"}
		]}`)
	}))
	defer ts.Close()

	got, err := pickOllamaModel(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got != "new:latest" {
		t.Errorf("got %q, want new:latest", got)
	}
}

func TestPickOllamaModelEmptyList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[]}`)
	}))
	defer ts.Close()

	if _, err := pickOllamaModel(ts.URL); err == nil {
		t.Fatal("expected error for empty model list")
	}
}

func TestResolveFlagsTakePrecedence(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "http://from-env/v1")
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("OPENAI_MODEL", "env-model")

	cfg, err := Resolve(Options{
		Endpoint: "http://flag/v1",
		APIKey:   "flag-key",
		Model:    "flag-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endpoint != "http://flag/v1" || cfg.APIKey != "flag-key" || cfg.Model != "flag-model" {
		t.Errorf("flags should win: %+v", cfg)
	}
}

func TestResolveFromEnv(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "http://env/v1")
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("OPENAI_MODEL", "env-model")
	t.Setenv("OLLAMA_HOST", "")

	cfg, err := Resolve(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endpoint != "http://env/v1" || cfg.Model != "env-model" || cfg.APIKey != "env-key" {
		t.Errorf("env values not used: %+v", cfg)
	}
}

func TestResolveAutodiscoversFromOllama(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"models":[{"name":"shiny:latest","modified_at":"2026-04-26T00:00:00Z"}]}`)
	}))
	defer ts.Close()

	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_MODEL", "")
	t.Setenv("OLLAMA_HOST", ts.URL)

	cfg, err := Resolve(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endpoint != ts.URL+"/v1" {
		t.Errorf("endpoint = %q, want %s/v1", cfg.Endpoint, ts.URL)
	}
	if cfg.Model != "shiny:latest" {
		t.Errorf("model = %q, want shiny:latest", cfg.Model)
	}
}
