package specmode

import (
	"context"
	"strings"
	"testing"

	"github.com/boytegar/packboy-builder/internal/tool"
)

func TestExitSpecModeRejectsEmptyPlan(t *testing.T) {
	esm := NewExitSpecModeTool()
	_, err := esm.PrepareInteraction(context.Background(), map[string]any{
		"plan": "",
	}, "/repo")
	if err == nil {
		t.Fatal("expected empty plan to be rejected")
	}
	if !strings.Contains(err.Error(), "plan is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExitSpecModePreparesApprovalQuestion(t *testing.T) {
	esm := NewExitSpecModeTool()
	req, err := esm.PrepareInteraction(context.Background(), map[string]any{
		"plan":  "1. Step one\n2. Step two",
		"title": "My Plan",
	}, "/repo")
	if err != nil {
		t.Fatalf("PrepareInteraction() error: %v", err)
	}

	qr, ok := req.(*tool.QuestionRequest)
	if !ok {
		t.Fatalf("expected *QuestionRequest, got %T", req)
	}
	if len(qr.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qr.Questions))
	}
	q := qr.Questions[0]
	if len(q.Options) != 2 {
		t.Fatalf("expected 2 options (Approve/Reject), got %d", len(q.Options))
	}
	if q.Options[0].Label != "Approve" {
		t.Errorf("first option = %q, want Approve", q.Options[0].Label)
	}
	if q.Options[1].Label != "Reject" {
		t.Errorf("second option = %q, want Reject", q.Options[1].Label)
	}
}

func TestExitSpecModeApproveReturnsSuccess(t *testing.T) {
	esm := NewExitSpecModeTool()
	params := map[string]any{
		"plan":  "1. Implement feature X",
		"title": "Feature X",
	}
	req, _ := esm.PrepareInteraction(context.Background(), params, "/repo")
	qr := req.(*tool.QuestionRequest)

	resp := &tool.QuestionResponse{
		RequestID: qr.ID,
		Answers:   map[int][]string{0: {"Approve"}},
	}

	result := esm.ExecuteWithResponse(context.Background(), params, resp, "/repo")
	if !result.Success {
		t.Error("expected success on Approve")
	}
	if !strings.Contains(result.Output, "approved") {
		t.Errorf("expected output to mention approval, got: %s", result.Output)
	}
	hook, ok := result.HookResponse.(map[string]any)
	if !ok {
		t.Fatal("expected HookResponse to be a map")
	}
	if hook["specApproved"] != true {
		t.Error("expected specApproved=true in HookResponse")
	}
	if hook["exitSpecMode"] != true {
		t.Error("expected exitSpecMode=true in HookResponse")
	}
}

func TestExitSpecModeRejectReturnsFailure(t *testing.T) {
	esm := NewExitSpecModeTool()
	params := map[string]any{
		"plan": "1. Some plan",
	}
	req, _ := esm.PrepareInteraction(context.Background(), params, "/repo")
	qr := req.(*tool.QuestionRequest)

	resp := &tool.QuestionResponse{
		RequestID: qr.ID,
		Answers:   map[int][]string{0: {"Reject"}},
	}

	result := esm.ExecuteWithResponse(context.Background(), params, resp, "/repo")
	if result.Success {
		t.Error("expected failure on Reject")
	}
	hook, ok := result.HookResponse.(map[string]any)
	if !ok {
		t.Fatal("expected HookResponse to be a map")
	}
	if hook["specApproved"] != false {
		t.Error("expected specApproved=false in HookResponse")
	}
	if hook["exitSpecMode"] != false {
		t.Error("expected exitSpecMode=false in HookResponse")
	}
}

func TestExitSpecModeCancelled(t *testing.T) {
	esm := NewExitSpecModeTool()
	params := map[string]any{
		"plan": "1. Some plan",
	}
	req, _ := esm.PrepareInteraction(context.Background(), params, "/repo")
	qr := req.(*tool.QuestionRequest)

	resp := &tool.QuestionResponse{
		RequestID: qr.ID,
		Cancelled: true,
	}

	result := esm.ExecuteWithResponse(context.Background(), params, resp, "/repo")
	if result.Success {
		t.Error("expected failure on Cancel")
	}
}

func TestExitSpecModeDefaultTitle(t *testing.T) {
	esm := NewExitSpecModeTool()
	params := map[string]any{
		"plan": "1. Some plan",
	}
	req, err := esm.PrepareInteraction(context.Background(), params, "/repo")
	if err != nil {
		t.Fatalf("PrepareInteraction() error: %v", err)
	}
	qr := req.(*tool.QuestionRequest)
	resp := &tool.QuestionResponse{
		RequestID: qr.ID,
		Answers:   map[int][]string{0: {"Approve"}},
	}
	result := esm.ExecuteWithResponse(context.Background(), params, resp, "/repo")
	hook := result.HookResponse.(map[string]any)
	if hook["title"] != "Implementation Plan" {
		t.Errorf("expected default title, got %v", hook["title"])
	}
}

func TestExitSpecModeRequiresInteraction(t *testing.T) {
	esm := NewExitSpecModeTool()
	if !esm.RequiresInteraction() {
		t.Error("expected RequiresInteraction() to be true")
	}
	if esm.RequiresPermission() {
		t.Error("expected RequiresPermission() to be false")
	}
}

func TestExitSpecModeExecuteDirect(t *testing.T) {
	esm := NewExitSpecModeTool()
	result := esm.Execute(context.Background(), map[string]any{"plan": "x"}, "/repo")
	if result.Success {
		t.Error("direct Execute should fail — requires interaction")
	}
}

func TestExitSpecModeRegistered(t *testing.T) {
	tool, ok := tool.Get("ExitSpecMode")
	if !ok {
		t.Fatal("ExitSpecMode tool not registered")
	}
	if tool.Name() != "ExitSpecMode" {
		t.Errorf("tool name = %q, want ExitSpecMode", tool.Name())
	}
}
