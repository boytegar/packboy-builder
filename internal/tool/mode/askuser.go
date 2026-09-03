package mode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/boytegar/packboy-builder/internal/tool"
	"github.com/boytegar/packboy-builder/internal/tool/toolresult"
)

type AskUserQuestionTool struct {
	requestCounter int
}

const (
	maxAskUserQuestions = 8
	maxAskUserOptions   = 8
)

func NewAskUserQuestionTool() *AskUserQuestionTool {
	return &AskUserQuestionTool{}
}

func (t *AskUserQuestionTool) Name() string {
	return "AskUserQuestion"
}

func (t *AskUserQuestionTool) Description() string {
	return "Ask the user questions to gather preferences, clarify requirements, or get decisions on implementation choices."
}

func (t *AskUserQuestionTool) Icon() string {
	return "❓"
}

func (t *AskUserQuestionTool) RequiresInteraction() bool {
	return true
}

// inputOption supports both simple string options and structured options
// with label + description.
type inputOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// inputQuestion supports the expanded questionnaire format:
// question, topic, multi, options (string[] or {label, description}[]).
type inputQuestion struct {
	Question string        `json:"question"`
	Topic    string        `json:"topic,omitempty"`
	Multi    bool          `json:"multi,omitempty"`
	Options  []inputOption `json:"options,omitempty"`

	// Legacy: raw string options (when the caller passes ["A","B"] instead of
	// [{"label":"A"},{"label":"B"}]). Parsed into Options above.
	RawOptions []string `json:"-"`
}

// rawInputQuestion is used for JSON unmarshalling because options can be
// either strings or objects.
type rawInputQuestion struct {
	Question string          `json:"question"`
	Topic    string          `json:"topic,omitempty"`
	Multi    bool            `json:"multi,omitempty"`
	Options  json.RawMessage `json:"options,omitempty"`
}

func parseInput(params map[string]any) ([]inputQuestion, error) {
	if questionsRaw, ok := params["questions"]; ok {
		return parseQuestionsArray(questionsRaw)
	}

	// Single-question shortcut: question + options at top level
	q, _ := params["question"].(string)
	if q == "" {
		q = "Please choose:"
	}
	topic, _ := params["topic"].(string)
	multi, _ := params["multi"].(bool)

	opts, err := parseOptionsValue(params["options"])
	if err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return nil, fmt.Errorf("missing required parameter: options (or questions)")
	}

	return []inputQuestion{{
		Question: q,
		Topic:    topic,
		Multi:    multi,
		Options:  opts,
	}}, nil
}

func parseQuestionsArray(questionsRaw any) ([]inputQuestion, error) {
	data, err := json.Marshal(questionsRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid questions format: %w", err)
	}

	var raw []rawInputQuestion
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("questions must be an array of {question, options, topic?, multi?}: %w", err)
	}

	result := make([]inputQuestion, 0, len(raw))
	for _, r := range raw {
		opts, err := parseOptionsRaw(r.Options)
		if err != nil {
			return nil, err
		}
		if len(opts) == 0 {
			return nil, fmt.Errorf("question %q: options is required", r.Question)
		}
		result = append(result, inputQuestion{
			Question: r.Question,
			Topic:    r.Topic,
			Multi:    r.Multi,
			Options:  opts,
		})
	}
	return result, nil
}

func parseOptionsValue(val any) ([]inputOption, error) {
	if val == nil {
		return nil, nil
	}
	data, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("invalid options format: %w", err)
	}
	return parseOptionsRaw(data)
}

func parseOptionsRaw(data json.RawMessage) ([]inputOption, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "[") {
		// Try string array first
		var strs []string
		if err := json.Unmarshal(data, &strs); err == nil {
			opts := make([]inputOption, len(strs))
			for i, s := range strs {
				opts[i] = inputOption{Label: s}
			}
			return opts, nil
		}
		// Fall back to structured objects
		var objs []inputOption
		if err := json.Unmarshal(data, &objs); err != nil {
			return nil, fmt.Errorf("options must be an array of strings or {label, description} objects: %w", err)
		}
		return objs, nil
	}
	return nil, fmt.Errorf("options must be an array")
}

func (t *AskUserQuestionTool) PrepareInteraction(ctx context.Context, params map[string]any, cwd string) (any, error) {
	input, err := parseInput(params)
	if err != nil {
		return nil, err
	}

	if len(input) == 0 || len(input) > maxAskUserQuestions {
		return nil, fmt.Errorf("must have 1-%d questions, got %d", maxAskUserQuestions, len(input))
	}

	questions := make([]tool.Question, len(input))
	for i, q := range input {
		if strings.TrimSpace(q.Question) == "" {
			return nil, fmt.Errorf("question[%d]: question text is required", i)
		}
		if len(q.Options) < 2 || len(q.Options) > maxAskUserOptions {
			return nil, fmt.Errorf("question[%d]: must have 2-%d options, got %d", i, maxAskUserOptions, len(q.Options))
		}
		opts := make([]tool.QuestionOption, len(q.Options))
		for j, o := range q.Options {
			if strings.TrimSpace(o.Label) == "" {
				return nil, fmt.Errorf("question[%d].options[%d]: label must not be empty", i, j)
			}
			opts[j] = tool.QuestionOption{
				Label:       o.Label,
				Description: o.Description,
			}
		}

		// Derive header from topic or question number
		header := fmt.Sprintf("Q%d", i+1)
		if len(input) == 1 {
			header = "Choose"
		}
		if q.Topic != "" {
			header = q.Topic
		}

		questions[i] = tool.Question{
			Question:    q.Question,
			Header:      header,
			Topic:       q.Topic,
			Options:     opts,
			MultiSelect: q.Multi,
		}
	}

	t.requestCounter++
	return &tool.QuestionRequest{
		ID:        fmt.Sprintf("ask-%d", t.requestCounter),
		Questions: questions,
	}, nil
}

func (t *AskUserQuestionTool) ExecuteWithResponse(ctx context.Context, params map[string]any, response any, cwd string) toolresult.ToolResult {
	resp, ok := response.(*tool.QuestionResponse)
	if !ok {
		return toolresult.NewErrorResult("AskUserQuestion", "invalid response type")
	}

	if resp.Cancelled {
		return toolresult.ToolResult{
			Success: true,
			Output:  "User cancelled the question prompt without answering.",
			Metadata: toolresult.ResultMetadata{
				Title:    "AskUserQuestion",
				Icon:     "❓",
				Subtitle: "Cancelled",
			},
		}
	}

	input, _ := parseInput(params)

	var parts []string
	for i, answers := range resp.Answers {
		if i >= len(input) {
			break
		}
		sel := strings.Join(answers, ", ")
		if sel == "" {
			sel = "(no selection)"
		}
		// Include topic if present
		q := input[i]
		if q.Topic != "" {
			parts = append(parts, fmt.Sprintf("[%s] %s → %s", q.Topic, q.Question, sel))
		} else {
			parts = append(parts, fmt.Sprintf("%s → %s", q.Question, sel))
		}
	}

	if len(parts) == 0 {
		return toolresult.ToolResult{
			Success:  true,
			Output:   "User did not select any option.",
			Metadata: toolresult.ResultMetadata{Title: "AskUserQuestion", Icon: "❓", Subtitle: "No selection"},
		}
	}

	output := "User responses:\n" + strings.Join(parts, "\n")
	subtitle := strings.Join(parts, "; ")
	if len(subtitle) > 60 {
		subtitle = subtitle[:57] + "..."
	}
	return toolresult.ToolResult{
		Success: true,
		Output:  output,
		Metadata: toolresult.ResultMetadata{
			Title:    "AskUserQuestion",
			Icon:     "❓",
			Subtitle: subtitle,
		},
	}
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, params map[string]any, cwd string) toolresult.ToolResult {
	return toolresult.NewErrorResult("AskUserQuestion", "this tool requires user interaction - use PrepareInteraction and ExecuteWithResponse")
}

func init() {
	tool.Register(NewAskUserQuestionTool())
}
