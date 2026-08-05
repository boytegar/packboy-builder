package input

import (
	"testing"

	"github.com/boytegar/packboy-builder/internal/core"
)

// TestImageLabelTruncatedFilename verifies the inline token shape is
// "[<truncated-filename> #<id>]".
func TestImageLabelTruncatedFilename(t *testing.T) {
	cases := []struct {
		id       int
		fileName string
		want     string
	}{
		{1, "inigambarsaya.png", "[iniga-.png #1]"},
		{2, "hello.png", "[hello-.png #2]"},
		{3, "clipboard_123456.png", "[clipb-.png #3]"},
		{10, "ab.jpeg", "[ab-.jpeg #10]"},
	}
	for _, c := range cases {
		got := imageLabel(c.id, c.fileName)
		if got != c.want {
			t.Errorf("imageLabel(%d, %q) = %q, want %q", c.id, c.fileName, got, c.want)
		}
	}
}

// TestImageLabelMatchesRegex verifies the new token shape is matched by
// core.InlineImageTokenRe (backward-compatible regex with ID in group 1).
func TestImageLabelMatchesRegex(t *testing.T) {
	label := imageLabel(7, "inigambarsaya.png")
	matches := core.InlineImageTokenRe.FindStringSubmatch(label)
	if matches == nil {
		t.Fatalf("regex did not match label %q", label)
	}
	if matches[1] != "7" {
		t.Errorf("regex group 1 (id) = %q, want %q for label %q", matches[1], "7", label)
	}
}

// TestLegacyTokenStillMatchesRegex verifies the old "[Image #N]" shape still
// matches (backward compat for resumed sessions).
func TestLegacyTokenStillMatchesRegex(t *testing.T) {
	if !core.InlineImageTokenRe.MatchString("[Image #1]") {
		t.Error("legacy [Image #1] token no longer matches regex")
	}
	if !core.InlineImageTokenRe.MatchString("[Image #10]") {
		t.Error("legacy [Image #10] token no longer matches regex")
	}
}

// TestAddPendingImageReturnsTruncatedLabel verifies the label returned by
// AddPendingImage carries the truncated filename + id.
func TestAddPendingImageReturnsTruncatedLabel(t *testing.T) {
	m := New("", 80, nil, SelectorDeps{})
	label := m.AddPendingImage(core.Image{FileName: "inigambarsaya.png"})
	want := "[iniga-.png #1]"
	if label != want {
		t.Errorf("AddPendingImage label = %q, want %q", label, want)
	}
}
