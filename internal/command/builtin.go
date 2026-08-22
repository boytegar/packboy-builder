package command

import (
	_ "embed"
	"fmt"
)

// WrapInvocation envelopes a workflow body in the <custom-command> tag expected
// by the skill-invocation pipeline. Centralizing the envelope keeps
// user-defined custom commands consistent.
func WrapInvocation(name, body string) string {
	return fmt.Sprintf("<custom-command name=%q>\n%s\n</custom-command>", name, body)
}

//go:embed prompts/simplify.md
var simplifyPrompt string

//go:embed prompts/plan.md
var planPrompt string

//go:embed prompts/graph.md
var graphPrompt string

//go:embed prompts/kg-tutor.md
var kgTutorPrompt string

//go:embed prompts/kg-scope.md
var kgScopePrompt string

//go:embed prompts/kg-schema.md
var kgSchemaPrompt string

//go:embed prompts/kg-extract.md
var kgExtractPrompt string

//go:embed prompts/kg-relations.md
var kgRelationsPrompt string

//go:embed prompts/kg-events.md
var kgEventsPrompt string

//go:embed prompts/kg-fuse.md
var kgFusePrompt string

//go:embed prompts/kg-eval.md
var kgEvalPrompt string

//go:embed prompts/kg-rag.md
var kgRagPrompt string

//go:embed prompts/context-generate.md
var contextGeneratePrompt string

// ContextGeneratePrompt returns the embedded PROJECT_CONTEXT_ARCHITECT workflow body
// used by /init to generate AGENTS.md and the .agents/ documentation set.
func ContextGeneratePrompt() string { return contextGeneratePrompt }

// builtinPromptCommands are slash commands that ship with Packboy Builder as embedded
// markdown workflows rather than Go handlers. They dispatch through the same
// <custom-command> pipeline as user-defined commands, and a user or project
// command with the same name takes precedence — customizing a shipped
// workflow is just dropping a file into .pcb/commands/.
func builtinPromptCommands() []CustomCommand {
	return []CustomCommand{
		{
			Name:        "simplify",
			Description: "Review the changed code with 4 parallel cleanup agents (reuse, simplification, efficiency, altitude), then apply the fixes",
			Body:        simplifyPrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "plan",
			Description: "Plan a feature/change — gather knowledge to ≥90% confidence, ask clarifying questions if below threshold, then write a grounded plan to notes/active/",
			Body:        planPrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "graph",
			Description: "Graph engineering entry point — knowledge graphs (ontology, extraction, fusion, GraphRAG) + task graphs (diamond, stop rule, human gate). Routes to /kg-* workflows",
			Body:        graphPrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "kg-tutor",
			Description: "Learn the 9-stage knowledge graph pipeline interactively (scope → ontology → extraction → fusion → GraphRAG)",
			Body:        kgTutorPrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "kg-scope",
			Description: "Model a domain before writing code — entity types, relation types, competency questions",
			Body:        kgScopePrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "kg-schema",
			Description: "Turn a scope draft into a real ontology — class hierarchy, domain/range, Turtle serialization",
			Body:        kgSchemaPrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "kg-extract",
			Description: "Design the extraction pipeline — source routing, chunking, failure modes, hand-check protocol",
			Body:        kgExtractPrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "kg-relations",
			Description: "Typed triple extraction with provenance — prompt, distant-supervision baseline, rejection rules",
			Body:        kgRelationsPrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "kg-events",
			Description: "Model events as first-class nodes — trigger, arguments, time anchor, causal/temporal edges",
			Body:        kgEventsPrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "kg-fuse",
			Description: "Entity resolution — blocking, match function, review band, reversible merge policy",
			Body:        kgFusePrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "kg-eval",
			Description: "Skeptical review — precision/recall at triple level, test-set leakage, link-prediction filtering",
			Body:        kgEvalPrompt,
			Scope:       scopeBuiltin,
		},
		{
			Name:        "kg-rag",
			Description: "Wire the graph into an agent — retrieval strategy per question type, vector baseline, 30-question eval set",
			Body:        kgRagPrompt,
			Scope:       scopeBuiltin,
		},
	}
}
