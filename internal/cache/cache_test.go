package cache

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"llama3.2:latest", "llama3.2-latest"},
		{"nemotron-3-nano:30b-a3b-fp16", "nemotron-3-nano-30b-a3b-fp16"},
		{"gpt-4o", "gpt-4o"},
		{"!!!model///name", "model-name"},
		{"  spaces  ", "spaces"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Slug(c.in); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInputKeyDeterministic(t *testing.T) {
	if InputKey("hello") != InputKey("hello") {
		t.Fatal("InputKey not deterministic")
	}
	if InputKey("hello") == InputKey("hello ") {
		t.Fatal("InputKey ignores whitespace differences")
	}
	if got := len(InputKey("x")); got != 64 {
		t.Errorf("hex length = %d, want 64", got)
	}
}

func TestPutAndVariantsRoundtrip(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := InputKey("the input text")
	want := Entry{
		Model:       "llama3.2:latest",
		Endpoint:    "http://localhost:11434/v1",
		Style:       "basic",
		Created:     time.Date(2026, 4, 26, 12, 34, 56, 0, time.UTC),
		Elapsed:     2500 * time.Millisecond,
		InputBytes:  1234,
		OutputBytes: 18,
		Body:        "this is\na summary",
	}
	path, err := c.Put(key, want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, ".md") {
		t.Errorf("path missing .md: %s", path)
	}
	if !strings.Contains(filepath.Base(path), "llama3.2-latest__basic__") {
		t.Errorf("filename does not encode model/style: %s", path)
	}

	got, err := c.Variants(key, "llama3.2:latest", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Variants returned %d entries, want 1", len(got))
	}
	g := got[0]
	if g.Body != want.Body {
		t.Errorf("body roundtrip: got %q, want %q", g.Body, want.Body)
	}
	if g.Model != want.Model || g.Endpoint != want.Endpoint || g.Style != want.Style {
		t.Errorf("metadata roundtrip mismatch: %+v vs %+v", g, want)
	}
	if !g.Created.Equal(want.Created) {
		t.Errorf("created: got %v, want %v", g.Created, want.Created)
	}
	if g.Elapsed != want.Elapsed {
		t.Errorf("elapsed: got %v, want %v", g.Elapsed, want.Elapsed)
	}
	if g.InputBytes != want.InputBytes || g.OutputBytes != want.OutputBytes {
		t.Errorf("byte counts roundtrip mismatch: %+v vs %+v", g, want)
	}
}

func TestPutDeduplicatesIdenticalBody(t *testing.T) {
	c, _ := New(t.TempDir())
	key := InputKey("dup")
	e := Entry{Model: "m", Style: "s", Body: "same body"}
	p1, _ := c.Put(key, e)
	p2, _ := c.Put(key, e)
	if p1 != p2 {
		t.Errorf("identical body should produce same path: %s vs %s", p1, p2)
	}
	all, err := c.All(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 entry after dedupe, got %d", len(all))
	}
}

func TestVariantsFiltersByModelAndStyle(t *testing.T) {
	c, _ := New(t.TempDir())
	key := InputKey("text")
	c.Put(key, Entry{Model: "llama", Style: "basic", Body: "L1"})
	c.Put(key, Entry{Model: "llama", Style: "basic", Body: "L2"})
	c.Put(key, Entry{Model: "gpt-4", Style: "basic", Body: "G1"})
	c.Put(key, Entry{Model: "llama", Style: "article", Body: "A1"})

	got, err := c.Variants(key, "llama", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 llama+basic variants, got %d (%+v)", len(got), got)
	}
	bodies := map[string]bool{got[0].Body: true, got[1].Body: true}
	if !bodies["L1"] || !bodies["L2"] {
		t.Errorf("missing expected bodies: %+v", got)
	}
}

func TestVariantsTreatsModelArgAsRawNotSlugged(t *testing.T) {
	// Caller passes the original model name; the cache slugs it internally.
	c, _ := New(t.TempDir())
	key := InputKey("text")
	c.Put(key, Entry{Model: "llama3.2:latest", Style: "s", Body: "x"})

	got, err := c.Variants(key, "llama3.2:latest", "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1, got %d", len(got))
	}
}

func TestAllSortsNewestFirst(t *testing.T) {
	c, _ := New(t.TempDir())
	key := InputKey("text")
	now := time.Now().UTC().Truncate(time.Second)
	c.Put(key, Entry{Model: "a", Style: "s", Created: now.Add(-2 * time.Hour), Body: "old"})
	c.Put(key, Entry{Model: "b", Style: "s", Created: now, Body: "new"})
	c.Put(key, Entry{Model: "c", Style: "s", Created: now.Add(-1 * time.Hour), Body: "mid"})

	got, err := c.All(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3", len(got))
	}
	if got[0].Body != "new" || got[1].Body != "mid" || got[2].Body != "old" {
		t.Errorf("not sorted newest first: %v / %v / %v", got[0].Body, got[1].Body, got[2].Body)
	}
}

func TestAllOnMissingDirReturnsNoError(t *testing.T) {
	c, _ := New(t.TempDir())
	got, err := c.All(InputKey("never written"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestParseEntryWithoutFrontMatter(t *testing.T) {
	got := parseEntry("just a body\nno front matter")
	if got.Body != "just a body\nno front matter" {
		t.Errorf("body: %q", got.Body)
	}
	if got.Model != "" || !got.Created.IsZero() {
		t.Errorf("expected zero metadata, got %+v", got)
	}
}

func TestParseEntryUnclosedFrontMatterTreatedAsBody(t *testing.T) {
	raw := "---\nmodel: x\nno closing marker"
	got := parseEntry(raw)
	if !strings.Contains(got.Body, "model: x") {
		t.Errorf("malformed FM should leave body intact, got %q", got.Body)
	}
}

func TestParseEntryBodyWithMarkdownHRSurvives(t *testing.T) {
	// Ensure a "---" inside the body (e.g. a markdown horizontal rule)
	// after proper front-matter closing is preserved.
	e := Entry{Model: "m", Style: "s", Body: "above\n\n---\n\nbelow"}
	raw := formatEntry(e)
	got := parseEntry(raw)
	if got.Body != e.Body {
		t.Errorf("body with HR not preserved: got %q, want %q", got.Body, e.Body)
	}
}
