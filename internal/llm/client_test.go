package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompleteHappyPath(t *testing.T) {
	var seen chatRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer testkey" {
			t.Errorf("auth header = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content-type = %q", got)
		}
		_ = json.NewDecoder(r.Body).Decode(&seen)
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"hi back"}}]}`)
	}))
	defer ts.Close()

	out, err := New(ts.URL, "testkey").Complete(context.Background(), "m", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if out != "hi back" {
		t.Errorf("got %q, want %q", out, "hi back")
	}
	if seen.Model != "m" {
		t.Errorf("request model = %q", seen.Model)
	}
	if len(seen.Messages) != 1 || seen.Messages[0].Role != "user" || seen.Messages[0].Content != "hello" {
		t.Errorf("request messages = %+v", seen.Messages)
	}
}

func TestCompleteNoAuthHeaderWhenKeyEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("expected no Authorization header, got %q", got)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer ts.Close()

	if _, err := New(ts.URL, "").Complete(context.Background(), "m", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteSurfacesAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"bad model"}}`)
	}))
	defer ts.Close()

	_, err := New(ts.URL, "").Complete(context.Background(), "m", "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad model") && !strings.Contains(err.Error(), "400") {
		t.Errorf("error message should reference status or message: %v", err)
	}
}

func TestCompleteEmptyChoices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[]}`)
	}))
	defer ts.Close()

	_, err := New(ts.URL, "").Complete(context.Background(), "m", "x")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestCompleteTrimsTrailingSlashOnEndpoint(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer ts.Close()

	if _, err := New(ts.URL+"/", "").Complete(context.Background(), "m", "x"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q (trailing slash should have been trimmed)", gotPath)
	}
}
