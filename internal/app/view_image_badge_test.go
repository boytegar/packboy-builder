package app

import (
	"strings"
	"testing"

	"github.com/boytegar/packboy-builder/internal/app/input"
	"github.com/boytegar/packboy-builder/internal/core"
)

// TestRenderImageBadgeLineEmpty verifies no badge line is rendered when no
// images are pending (so no blank line is inserted above the textarea).
func TestRenderImageBadgeLineEmpty(t *testing.T) {
	m := &model{
		userInput: input.New("", 80, nil, input.SelectorDeps{}),
	}
	if got := m.renderImageBadgeLine(); got != "" {
		t.Errorf("expected empty badge line with no pending images, got %q", got)
	}
}

// TestRenderImageBadgeLineWithImages verifies one badge per pending image,
// each mirroring the inline token label ([truncated-filename #id]).
func TestRenderImageBadgeLineWithImages(t *testing.T) {
	m := &model{
		userInput: input.New("", 80, nil, input.SelectorDeps{}),
	}
	m.userInput.AddPendingImage(core.Image{FileName: "inigambarsaya.png"})
	m.userInput.AddPendingImage(core.Image{FileName: "screenshot.jpeg"})

	got := m.renderImageBadgeLine()
	if !strings.Contains(got, "iniga-.png #1") {
		t.Errorf("expected first badge with truncated filename, got %q", got)
	}
	if !strings.Contains(got, "scree-.jpeg #2") {
		t.Errorf("expected second badge with truncated filename, got %q", got)
	}
	// Two badges separated by a space
	if strings.Count(got, "#") != 2 {
		t.Errorf("expected 2 badge tokens, got %d in %q", strings.Count(got, "#"), got)
	}
}
