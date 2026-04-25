// Package discover resolves the LLM endpoint and model from flags,
// environment, and (as a fallback) Ollama's HTTP API.
package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// Config is the resolved configuration.
type Config struct {
	Endpoint string // OpenAI-compatible base URL, e.g. http://localhost:11434/v1
	APIKey   string
	Model    string
}

// Options are user-provided overrides (typically from CLI flags).
type Options struct {
	Endpoint string
	APIKey   string
	Model    string
}

// Resolve fills in any unset fields in this order:
//
//  1. Options (CLI flags).
//  2. Environment: OPENAI_BASE_URL, OPENAI_API_KEY, OPENAI_MODEL.
//  3. Ollama autodiscovery via OLLAMA_HOST (default http://localhost:11434).
//     The endpoint becomes "<host>/v1" and the model is the most recently
//     modified non-embedding model returned by /api/tags.
func Resolve(o Options) (Config, error) {
	cfg := Config{Endpoint: o.Endpoint, APIKey: o.APIKey, Model: o.Model}

	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = os.Getenv("OPENAI_BASE_URL")
	}
	if cfg.Model == "" {
		cfg.Model = os.Getenv("OPENAI_MODEL")
	}
	if cfg.Endpoint != "" && cfg.Model != "" {
		return cfg, nil
	}

	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	host = normalizeHost(host)

	if cfg.Endpoint == "" {
		cfg.Endpoint = host + "/v1"
	}
	if cfg.Model == "" {
		m, err := pickOllamaModel(host)
		if err != nil {
			return cfg, fmt.Errorf("autodiscover model from %s: %w", host, err)
		}
		cfg.Model = m
	}
	return cfg, nil
}

// normalizeHost accepts "host:port", "http://host:port", or full URLs
// and returns a scheme-prefixed host with no trailing slash and no path.
func normalizeHost(h string) string {
	if !strings.HasPrefix(h, "http://") && !strings.HasPrefix(h, "https://") {
		h = "http://" + h
	}
	u, err := url.Parse(h)
	if err != nil {
		return strings.TrimRight(h, "/")
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

type ollamaTag struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
}

type ollamaTagsResponse struct {
	Models []ollamaTag `json:"models"`
}

func pickOllamaModel(host string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/tags", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s", resp.Status)
	}
	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", err
	}
	if len(tags.Models) == 0 {
		return "", fmt.Errorf("no models available")
	}
	sort.Slice(tags.Models, func(i, j int) bool {
		ei, ej := isEmbedding(tags.Models[i].Name), isEmbedding(tags.Models[j].Name)
		if ei != ej {
			return !ei
		}
		return tags.Models[i].ModifiedAt.After(tags.Models[j].ModifiedAt)
	})
	return tags.Models[0].Name, nil
}

func isEmbedding(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "embed") || strings.HasPrefix(n, "bge-") || strings.HasPrefix(n, "nomic-")
}
