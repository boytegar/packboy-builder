package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/boytegar/packboy-builder/internal/core"
)

// VisionAnalysisPrompt is the system prompt sent to the designated vision model
// when pre-analyzing images for a text-only main agent. It instructs the model
// to produce a factual, self-contained description of each image so the main
// agent can act on the image content without ever seeing the pixels.
const VisionAnalysisPrompt = `You are a vision pre-processor. The user has attached one or more images to their message, but the main agent that will answer is text-only and cannot see images. Your job is to analyze the images and return a concise, factual description of each so the main agent can reason about them.

For each image, describe:
- What the image depicts (objects, people, scenes, text on screen, diagrams, charts, UI elements).
- Any text visible in the image, transcribed verbatim where legible.
- Spatial relationships, layout, or structure that matters for understanding.
- Any notable details the user's message asks about, or that are otherwise relevant.

Return plain text, one section per image, each prefixed with "Image N:" on its own line (N = 1-based index in the order the images were provided). Be factual and precise — do not speculate beyond what is visible. Do not answer the user's question yourself; only describe what is in the images so the main agent can answer.`

// AnalyzeImages sends the given images to the designated vision model for
// pre-analysis and returns the model's text description. ref is a "vendor/model"
// or bare model id (same forms as SubagentDefaultModel); store resolves the
// provider. The images ride in a single core.UserMessage with Images set; the
// vision model sees the pixels, the main agent never does.
//
// Returns ("", nil) when there are no images to analyze. An error means the
// vision call failed and the caller should surface it rather than sending
// images to a text-only main model.
func AnalyzeImages(ctx context.Context, store *Store, ref string, images []core.Image) (string, error) {
	if len(images) == 0 {
		return "", nil
	}
	vendor, modelID, ok := ParseVendorModel(ref)
	if !ok {
		// Bare model id: stay on the store's current provider.
		cm := store.GetCurrentModel()
		if cm == nil {
			return "", fmt.Errorf("vision model %q is not a vendor/model ref and no current provider is set", ref)
		}
		vendor = cm.Provider
		modelID = ref
	}
	pool := NewProviderPool(store)
	provider, err := pool.Resolve(ctx, vendor)
	if err != nil {
		return "", fmt.Errorf("vision model provider %q unavailable: %w", vendor, err)
	}

	resp, err := Complete(ctx, provider, CompletionOptions{
		Model:        modelID,
		SystemPrompt: VisionAnalysisPrompt,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Analyze the attached image(s) and return a description per the instructions.", Images: images},
		},
		MaxTokens: 800,
	})
	if err != nil {
		return "", fmt.Errorf("vision analysis failed: %w", err)
	}
	// Some reasoning-heavy vision models (e.g. stepfun-3.7-flash on a custom
	// OpenAI-compat gateway) emit the description in the thinking/reasoning
	// channel with an empty Content. Fall back to that rather than silently
	// returning "" (which the caller treats as "no analysis").
	if out := strings.TrimSpace(resp.Content); out != "" {
		return out, nil
	}
	if out := strings.TrimSpace(resp.Thinking); out != "" {
		return out, nil
	}
	for _, ri := range resp.Reasoning {
		if r := strings.TrimSpace(ri.Summary); r != "" {
			return r, nil
		}
		if r := strings.TrimSpace(ri.EncryptedContent); r != "" {
			return r, nil
		}
	}
	return "", nil
}
