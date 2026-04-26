package prompts

import (
	"slices"
	"strings"
	"testing"
)

func TestStyleName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0000-basic.md", "basic"},
		{"0010-article.md", "article"},
		{"010-multi-word-name.md", "multi-word-name"},
		{"basic.md", "basic"},
		{"no-prefix-here.md", "no-prefix-here"},
		{"99-x.tmpl", "x"},
		{"-leading-dash.md", "-leading-dash"},
	}
	for _, c := range cases {
		if got := styleName(c.in); got != c.want {
			t.Errorf("styleName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestListContainsKnownStyles(t *testing.T) {
	got := List()
	for _, want := range []string{"article", "basic", "paper", "podcast", "transcript"} {
		if !slices.Contains(got, want) {
			t.Errorf("List() missing %q (got %v)", want, got)
		}
	}
	if !slices.IsSorted(got) {
		t.Errorf("List() not sorted: %v", got)
	}
}

func TestRenderSubstitutesText(t *testing.T) {
	out, err := Render("basic", Data{Text: "HELLO_WORLD_TOKEN"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "HELLO_WORLD_TOKEN") {
		t.Errorf("Render did not substitute Text in %q", out)
	}
}

func TestRenderUnknownStyleErrors(t *testing.T) {
	_, err := Render("does-not-exist-xyz", Data{Text: "x"})
	if err == nil {
		t.Fatal("expected error for unknown style")
	}
	if !strings.Contains(err.Error(), "does-not-exist-xyz") {
		t.Errorf("error should mention the bad style: %v", err)
	}
}
