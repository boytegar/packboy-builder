package mode

import "github.com/boytegar/packboy-builder/internal/core"

func (t *AskUserQuestionTool) Schema() core.ToolSchema {
	return core.ToolSchema{
		Name: "AskUserQuestion",
		Description: `Ask the user a questionnaire with predefined choices; an 'Other' free-text option is appended automatically.

Use this instead of plain text when you need a decision or preference (ambiguous request, multiple valid approaches); skip it when a reasonable default exists or the answer is in the code.

Key rules:
- 1-4 questions per call; 2-4 options per question (multi-select questions allow more).
- Keep option labels short (2-3 words max). The UI shows an 'Other' option automatically.
- Mark a question as "multi": true for select-all-that-apply.
- Use "topic" for short navigation labels (e.g. "Library", "Features") — multi-word topics are normalized.
- Understandable on its own: repeat essential context in each question.

Single question (most common):
  {"question": "Which version?", "options": ["v1.0", "v2.0", "v3.0"]}

Single multi-select:
  {"question": "Which features?", "multi": true, "options": ["Auth", "Login", "Dashboard"]}

Single question with topic + structured options:
  {"question": "Which approach?", "topic": "Design", "options": [{"label": "Minimal", "description": "Clean monochrome"}, {"label": "Bold", "description": "High contrast"}]}

Multiple questions (rare):
  {"questions": [{"question": "Q1?", "topic": "Topic1", "options": ["A","B"]}, {"question": "Q2?", "topic": "Topic2", "multi": true, "options": ["X","Y"]}]}`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{
					"type":        "string",
					"description": "The question text (for single question)",
				},
				"topic": map[string]any{
					"type":        "string",
					"description": "Short label for the question, used for navigation (e.g. 'Library', 'Features'). Multi-word topics are normalized to a nav label.",
				},
				"multi": map[string]any{
					"type":        "boolean",
					"description": "If true, the user may select multiple options (select all that apply). Default: false (single choice).",
				},
				"options": map[string]any{
					"type":        "array",
					"description": "2-8 choice labels (for single question). Can be strings or {label, description} objects.",
					"minItems":    2,
					"maxItems":    8,
					"items": map[string]any{
						"oneOf": []map[string]any{
							{"type": "string"},
							{
								"type": "object",
								"properties": map[string]any{
									"label":       map[string]any{"type": "string"},
									"description": map[string]any{"type": "string"},
								},
								"required": []string{"label"},
							},
						},
					},
				},
				"questions": map[string]any{
					"type":        "array",
					"description": "For multiple questions. Array of {question, topic?, multi?, options} objects (max 8).",
					"minItems":    1,
					"maxItems":    8,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"question": map[string]any{"type": "string"},
							"topic": map[string]any{
								"type":        "string",
								"description": "Short nav label for this question.",
							},
							"multi": map[string]any{
								"type":        "boolean",
								"description": "Allow multiple selections for this question.",
							},
							"options": map[string]any{
								"type":     "array",
								"minItems": 2,
								"maxItems": 8,
								"items": map[string]any{
									"oneOf": []map[string]any{
										{"type": "string"},
										{
											"type": "object",
											"properties": map[string]any{
												"label":       map[string]any{"type": "string"},
												"description": map[string]any{"type": "string"},
											},
											"required": []string{"label"},
										},
									},
								},
							},
						},
						"required": []string{"question", "options"},
					},
				},
			},
			"minProperties": 1,
			"required":      []string{},
		},
	}
}
