package mode

import (
	"context"
	"strings"
	"testing"

	"github.com/boytegar/packboy-builder/internal/tool"
)

func TestAskUserQuestionRejectsEmptyQuestions(t *testing.T) {
	ask := NewAskUserQuestionTool()
	_, err := ask.PrepareInteraction(context.Background(), map[string]any{
		"questions": []any{},
	}, "/repo")
	if err == nil {
		t.Fatal("expected empty questions to be rejected")
	}
	if !strings.Contains(err.Error(), "must have 1-8 questions") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAskUserQuestionPreparesSingleQuestionWithEightOptions(t *testing.T) {
	ask := NewAskUserQuestionTool()
	req, err := ask.PrepareInteraction(context.Background(), map[string]any{
		"question": "What version should I release?",
		"options":  []string{"v1.15.2", "v1.16.0", "v2.0.0", "patch", "minor", "major", "dry run", "cancel"},
	}, "/repo")
	if err != nil {
		t.Fatalf("PrepareInteraction() error: %v", err)
	}

	got := req.(*tool.QuestionRequest).Questions
	if len(got) != 1 {
		t.Fatalf("expected 1 question, got %d", len(got))
	}
	if len(got[0].Options) != 8 {
		t.Fatalf("expected 8 options, got %d", len(got[0].Options))
	}
}

func TestAskUserQuestionParsesTopic(t *testing.T) {
	ask := NewAskUserQuestionTool()
	req, err := ask.PrepareInteraction(context.Background(), map[string]any{
		"question": "Which library?",
		"topic":    "Library",
		"options":  []string{"LibA", "LibB"},
	}, "/repo")
	if err != nil {
		t.Fatalf("PrepareInteraction() error: %v", err)
	}
	got := req.(*tool.QuestionRequest).Questions[0]
	if got.Topic != "Library" {
		t.Errorf("topic = %q, want Library", got.Topic)
	}
	if got.Header != "Library" {
		t.Errorf("header = %q, want Library (derived from topic)", got.Header)
	}
}

func TestAskUserQuestionParsesMultiSelect(t *testing.T) {
	ask := NewAskUserQuestionTool()
	req, err := ask.PrepareInteraction(context.Background(), map[string]any{
		"question": "Which features?",
		"multi":    true,
		"options":  []string{"Auth", "Login", "Dashboard"},
	}, "/repo")
	if err != nil {
		t.Fatalf("PrepareInteraction() error: %v", err)
	}
	got := req.(*tool.QuestionRequest).Questions[0]
	if !got.MultiSelect {
		t.Error("expected MultiSelect=true")
	}
}

func TestAskUserQuestionParsesStructuredOptions(t *testing.T) {
	ask := NewAskUserQuestionTool()
	req, err := ask.PrepareInteraction(context.Background(), map[string]any{
		"question": "Which approach?",
		"options": []map[string]any{
			{"label": "Minimal", "description": "Clean monochrome"},
			{"label": "Bold", "description": "High contrast"},
		},
	}, "/repo")
	if err != nil {
		t.Fatalf("PrepareInteraction() error: %v", err)
	}
	got := req.(*tool.QuestionRequest).Questions[0]
	if len(got.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(got.Options))
	}
	if got.Options[0].Label != "Minimal" {
		t.Errorf("option[0] label = %q, want Minimal", got.Options[0].Label)
	}
	if got.Options[0].Description != "Clean monochrome" {
		t.Errorf("option[0] description = %q, want 'Clean monochrome'", got.Options[0].Description)
	}
}

func TestAskUserQuestionParsesMultiQuestionsWithTopicAndMulti(t *testing.T) {
	ask := NewAskUserQuestionTool()
	req, err := ask.PrepareInteraction(context.Background(), map[string]any{
		"questions": []map[string]any{
			{
				"question": "Q1?",
				"topic":    "Topic1",
				"options":  []string{"A", "B"},
			},
			{
				"question": "Q2?",
				"topic":    "Topic2",
				"multi":    true,
				"options":  []string{"X", "Y"},
			},
		},
	}, "/repo")
	if err != nil {
		t.Fatalf("PrepareInteraction() error: %v", err)
	}
	got := req.(*tool.QuestionRequest).Questions
	if len(got) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(got))
	}
	if got[0].Topic != "Topic1" {
		t.Errorf("q0 topic = %q, want Topic1", got[0].Topic)
	}
	if got[0].Header != "Topic1" {
		t.Errorf("q0 header = %q, want Topic1", got[0].Header)
	}
	if !got[1].MultiSelect {
		t.Error("expected q1 MultiSelect=true")
	}
}

func TestAskUserQuestionRejectsEmptyOptionLabel(t *testing.T) {
	ask := NewAskUserQuestionTool()
	_, err := ask.PrepareInteraction(context.Background(), map[string]any{
		"question": "Which?",
		"options":  []map[string]any{{"label": ""}, {"label": "B"}},
	}, "/repo")
	if err == nil {
		t.Fatal("expected error for empty label")
	}
}

func TestAskUserQuestionExecuteWithResponseIncludesTopic(t *testing.T) {
	ask := NewAskUserQuestionTool()
	params := map[string]any{
		"question": "Which?",
		"topic":    "Design",
		"options":  []string{"A", "B"},
	}
	req, _ := ask.PrepareInteraction(context.Background(), params, "/repo")
	qr := req.(*tool.QuestionRequest)
	resp := &tool.QuestionResponse{
		RequestID: qr.ID,
		Answers:   map[int][]string{0: {"A"}},
	}
	result := ask.ExecuteWithResponse(context.Background(), params, resp, "/repo")
	if !strings.Contains(result.Output, "[Design]") {
		t.Errorf("expected output to include topic [Design], got: %s", result.Output)
	}
}
