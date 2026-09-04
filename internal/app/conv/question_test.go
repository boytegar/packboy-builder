package conv

import (
	"strings"
	"testing"

	"github.com/boytegar/packboy-builder/internal/tool"
)

func TestQuestionPromptRenderUsesSingleOuterSeparators(t *testing.T) {
	p := NewQuestionPrompt()
	p.Show(&tool.QuestionRequest{
		ID: "ask-1",
		Questions: []tool.Question{{
			Question: "What version should I release?",
			Header:   "Choose",
			Options: []tool.QuestionOption{
				{Label: "Patch version"},
				{Label: "Minor version"},
			},
		}},
	}, 80)

	plain := stripANSI(p.Render())

	if strings.HasPrefix(plain, "─") {
		t.Fatalf("question prompt should rely on the modal wrapper for the top separator:\n%s", plain)
	}
	if !strings.HasSuffix(plain, strings.Repeat("─", 78)) {
		t.Fatalf("question prompt should end with a bottom separator:\n%s", plain)
	}
}

func TestQuestionPromptRenderWrapsLongQuestionTextToMultipleLines(t *testing.T) {
	longQuestion := "Which release strategy should we use for the upcoming major release that includes breaking API changes and migration guide?"
	p := NewQuestionPrompt()
	p.Show(&tool.QuestionRequest{
		ID: "ask-1",
		Questions: []tool.Question{{
			Question: longQuestion,
			Header:   "Choose",
			Options: []tool.QuestionOption{
				{Label: "Patch version"},
				{Label: "Minor version"},
			},
		}},
	}, 50)

	plain := stripANSI(p.Render())
	lines := strings.Split(plain, "\n")

	// The question text (after the header line) must occupy more than one line
	// when it exceeds the available width.
	// Layout: line 0 = header, lines 1..n = question text (wrapped), then blank + options.
	// Find the question text block: it follows the header line.
	var qLines []string
	for i, line := range lines {
		if strings.Contains(line, "Choose") {
			// Next lines until blank are the question text.
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					break
				}
				qLines = append(qLines, lines[j])
			}
			break
		}
	}

	if len(qLines) <= 1 {
		t.Fatalf("expected question text to wrap to multiple lines, got %d line(s):\n%s", len(qLines), plain)
	}

	// No wrapped line should exceed the available content width (50-2 = 48).
	for i, line := range qLines {
		if visibleWidth := len(strings.TrimRight(line, " ")); visibleWidth > 48 {
			t.Fatalf("wrapped line %d exceeds content width (%d > 48): %q", i, visibleWidth, line)
		}
	}
}

func TestQuestionPromptRenderDoesNotDuplicateOtherOption(t *testing.T) {
	p := NewQuestionPrompt()
	p.Show(&tool.QuestionRequest{
		ID: "ask-1",
		Questions: []tool.Question{{
			Question: "What version should I release?",
			Header:   "Choose",
			Options: []tool.QuestionOption{
				{Label: "Patch version"},
				{Label: "Minor version"},
				{Label: "Major version"},
				{Label: "Other"},
			},
		}},
	}, 80)

	plain := stripANSI(p.Render())
	if count := strings.Count(plain, "Other"); count != 1 {
		t.Fatalf("expected one Other option, got %d:\n%s", count, plain)
	}
	if !strings.Contains(plain, "4. Other - Type custom response") {
		t.Fatalf("existing Other option should be used for custom input:\n%s", plain)
	}
}
