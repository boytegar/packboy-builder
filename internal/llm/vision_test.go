package llm

import (
	"context"
	"testing"

	"github.com/boytegar/packboy-builder/internal/core"
)

// TestAnalyzeImagesNoImages verifies the helper short-circuits when no images.
func TestAnalyzeImagesNoImages(t *testing.T) {
	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	got, err := AnalyzeImages(context.Background(), store, "openai/gpt-4o", nil)
	if err != nil {
		t.Fatalf("expected no error with no images, got %v", err)
	}
	if got != "" {
		t.Errorf("expected empty analysis with no images, got %q", got)
	}
}

// TestAnalyzeImagesSuccess verifies a vision model call returns analysis text
// and that images are passed through to the provider.
func TestAnalyzeImagesSuccess(t *testing.T) {
	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	provider := &mockLLMProvider{
		responses: []CompletionResponse{{Content: "Image 1: a red square", StopReason: "end_turn"}},
	}
	RegisterProviderDisplay("testvision", ProviderDisplay{Name: "TestVision", Order: 999})
	Register(Meta{Provider: "testvision", AuthMethod: AuthAPIKey}, func(ctx context.Context) (Provider, error) {
		return provider, nil
	})
	defer globalRegistry.Unregister("testvision", AuthAPIKey)
	if err := store.Connect("testvision", AuthAPIKey); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	images := []core.Image{{FileName: "test.png", MediaType: "image/png", Data: "base64data"}}
	got, err := AnalyzeImages(context.Background(), store, "testvision/test-model", images)
	if err != nil {
		t.Fatalf("AnalyzeImages error: %v", err)
	}
	if got != "Image 1: a red square" {
		t.Errorf("analysis = %q, want %q", got, "Image 1: a red square")
	}
	if len(provider.lastOpts.Messages) == 0 || len(provider.lastOpts.Messages[0].Images) != 1 {
		t.Errorf("expected 1 image passed to provider, got %d messages", len(provider.lastOpts.Messages))
	}
}

// TestAnalyzeImagesProviderError verifies an unresolvable provider surfaces an error.
func TestAnalyzeImagesProviderError(t *testing.T) {
	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	_, err = AnalyzeImages(context.Background(), store, "nonexistent/model", []core.Image{{FileName: "x.png"}})
	if err == nil {
		t.Fatal("expected error for unresolvable provider, got nil")
	}
}

// TestAnalyzeImagesBareModelNoCurrent verifies a bare model id with no current
// model errors out.
func TestAnalyzeImagesBareModelNoCurrent(t *testing.T) {
	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	_, err = AnalyzeImages(context.Background(), store, "bare-model-id", []core.Image{{FileName: "x.png"}})
	if err == nil {
		t.Fatal("expected error for bare model id with no current provider, got nil")
	}
}
