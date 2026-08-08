package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TextDocumentItem mirrors LSP's textDocument/didOpen payload.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// Location is an LSP location (used by definition/references results).
type Location struct {
	URI   string   `json:"uri"`
	Range LSPRange `json:"range"`
}

// DefinitionResult is the textDocument/definition response.
type DefinitionResult struct {
	Locations []Location `json:"locations"`
}

// ReferencesParams is the textDocument/references payload.
type ReferencesParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     LSPPosition            `json:"position"`
	Context      ReferenceContext       `json:"context"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferencesResult is the textDocument/references response.
type ReferencesResult struct {
	Locations []Location `json:"locations"`
}

// OpenFile sends didOpen for a file and stores it in the open registry.
func (c *Client) OpenFile(ctx context.Context, uri, languageID, text string) error {
	item := TextDocumentItem{
		URI:        uri,
		LanguageID: languageID,
		Version:    1,
		Text:       text,
	}
	c.mu.Lock()
	if c.openVersions == nil {
		c.openVersions = make(map[string]int)
	}
	c.openVersions[uri] = 1
	c.mu.Unlock()
	return c.Notify(ctx, "textDocument/didOpen", item)
}

// ChangeFile sends didChange (full document) for a previously opened file.
func (c *Client) ChangeFile(ctx context.Context, uri, text string) error {
	c.mu.Lock()
	version := c.openVersions[uri]
	if version == 0 {
		c.mu.Unlock()
		return fmt.Errorf("lsp: file %q not open", uri)
	}
	c.openVersions[uri] = version + 1
	version++
	c.mu.Unlock()

	params := map[string]any{
		"textDocument":   TextDocumentIdentifier{URI: uri},
		"contentChanges": []any{map[string]any{"text": text}},
	}
	_ = version
	return c.Notify(ctx, "textDocument/didChange", params)
}

// CloseFile sends didClose for an open file.
func (c *Client) CloseFile(ctx context.Context, uri string) error {
	c.mu.Lock()
	delete(c.openVersions, uri)
	c.mu.Unlock()
	return c.Notify(ctx, "textDocument/didClose", map[string]any{
		"textDocument": TextDocumentIdentifier{URI: uri},
	})
}

// Definition requests the definition at a position.
func (c *Client) Definition(ctx context.Context, uri string, pos LSPPosition) (*DefinitionResult, error) {
	params := map[string]any{
		"textDocument": TextDocumentIdentifier{URI: uri},
		"position":     pos,
	}
	result, err := c.Send(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}
	var locs []Location
	if err := json.Unmarshal(result, &locs); err != nil {
		// Some servers return a single Location object instead of an array.
		var single Location
		if err2 := json.Unmarshal(result, &single); err2 != nil {
			return nil, fmt.Errorf("lsp: bad definition result: %w", err)
		}
		locs = []Location{single}
	}
	return &DefinitionResult{Locations: locs}, nil
}

// References requests all references to the symbol at a position.
func (c *Client) References(ctx context.Context, uri string, pos LSPPosition) (*ReferencesResult, error) {
	params := ReferencesParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
		Context:      ReferenceContext{IncludeDeclaration: true},
	}
	result, err := c.Send(ctx, "textDocument/references", params)
	if err != nil {
		return nil, err
	}
	var locs []Location
	if err := json.Unmarshal(result, &locs); err != nil {
		var single Location
		if err2 := json.Unmarshal(result, &single); err2 != nil {
			return nil, fmt.Errorf("lsp: bad references result: %w", err)
		}
		locs = []Location{single}
	}
	return &ReferencesResult{Locations: locs}, nil
}

// FileURI converts a filesystem path to an LSP file:// URI.
func FileURI(path string) string {
	return "file://" + filepath.ToSlash(path)
}

// LanguageFromPath guesses a languageId from the file extension, used for
// didOpen. Falls back to "plaintext".
func LanguageFromPath(path string) string {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "" {
		return "plaintext"
	}
	return ext
}

// ReadFileContent reads the current on-disk content of a path (used by tools
// before OpenFile / ChangeFile).
func ReadFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ---------- Document symbols ----------

// DocumentSymbol is an LSP document symbol (hierarchical).
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Kind           int              `json:"kind"`
	Range          LSPRange         `json:"range"`
	SelectionRange LSPRange         `json:"selectionRange"`
	Detail         string           `json:"detail,omitempty"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation is the flat (legacy) symbol response shape.
type SymbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      Location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

// DocumentSymbols requests the symbol tree for a document. Returns either
// hierarchical DocumentSymbol or flat SymbolInformation depending on server
// capabilities; both are returned in the combined slice.
func (c *Client) DocumentSymbols(ctx context.Context, uri string) ([]DocumentSymbol, []SymbolInformation, error) {
	params := map[string]any{
		"textDocument": TextDocumentIdentifier{URI: uri},
	}
	result, err := c.Send(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, nil, err
	}

	// Try hierarchical first.
	var hierarchical []DocumentSymbol
	if err := json.Unmarshal(result, &hierarchical); err == nil && len(hierarchical) > 0 {
		return hierarchical, nil, nil
	}

	// Fall back to flat SymbolInformation.
	var flat []SymbolInformation
	if err := json.Unmarshal(result, &flat); err != nil {
		return nil, nil, fmt.Errorf("lsp: bad documentSymbol result: %w", err)
	}
	return nil, flat, nil
}

// ---------- Call hierarchy ----------

// CallHierarchyItem is an LSP call hierarchy item.
type CallHierarchyItem struct {
	Name           string   `json:"name"`
	Kind           int      `json:"kind"`
	URI            string   `json:"uri"`
	Range          LSPRange `json:"range"`
	SelectionRange LSPRange `json:"selectionRange"`
	Detail         string   `json:"detail,omitempty"`
}

// CallHierarchyIncomingCall is an incoming call.
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []LSPRange        `json:"fromRanges"`
}

// CallHierarchyOutgoingCall is an outgoing call.
type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []LSPRange        `json:"fromRanges"`
}

// PrepareCallHierarchy prepares call hierarchy for a position.
func (c *Client) PrepareCallHierarchy(ctx context.Context, uri string, pos LSPPosition) ([]CallHierarchyItem, error) {
	params := map[string]any{
		"textDocument": TextDocumentIdentifier{URI: uri},
		"position":     pos,
	}
	result, err := c.Send(ctx, "textDocument/prepareCallHierarchy", params)
	if err != nil {
		return nil, err
	}
	var items []CallHierarchyItem
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, fmt.Errorf("lsp: bad prepareCallHierarchy result: %w", err)
	}
	return items, nil
}

// IncomingCalls requests incoming calls for a call hierarchy item.
func (c *Client) IncomingCalls(ctx context.Context, item CallHierarchyItem) ([]CallHierarchyIncomingCall, error) {
	params := map[string]any{"item": item}
	result, err := c.Send(ctx, "callHierarchy/incomingCalls", params)
	if err != nil {
		return nil, err
	}
	var calls []CallHierarchyIncomingCall
	if err := json.Unmarshal(result, &calls); err != nil {
		return nil, fmt.Errorf("lsp: bad incomingCalls result: %w", err)
	}
	return calls, nil
}

// OutgoingCalls requests outgoing calls for a call hierarchy item.
func (c *Client) OutgoingCalls(ctx context.Context, item CallHierarchyItem) ([]CallHierarchyOutgoingCall, error) {
	params := map[string]any{"item": item}
	result, err := c.Send(ctx, "callHierarchy/outgoingCalls", params)
	if err != nil {
		return nil, err
	}
	var calls []CallHierarchyOutgoingCall
	if err := json.Unmarshal(result, &calls); err != nil {
		return nil, fmt.Errorf("lsp: bad outgoingCalls result: %w", err)
	}
	return calls, nil
}

// ---------- Rename ----------

// RenameParams is the textDocument/rename payload.
type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     LSPPosition            `json:"position"`
	NewName      string                 `json:"newName"`
}

// WorkspaceEdit is the rename result.
type WorkspaceEdit struct {
	Changes         map[string][]LSPRange `json:"changes,omitempty"`
	DocumentChanges []any                 `json:"documentChanges,omitempty"`
}

// Rename requests a symbol rename at a position.
func (c *Client) Rename(ctx context.Context, uri string, pos LSPPosition, newName string) (*WorkspaceEdit, error) {
	params := RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
		NewName:      newName,
	}
	result, err := c.Send(ctx, "textDocument/rename", params)
	if err != nil {
		return nil, err
	}
	var edit WorkspaceEdit
	if err := json.Unmarshal(result, &edit); err != nil {
		return nil, fmt.Errorf("lsp: bad rename result: %w", err)
	}
	return &edit, nil
}

// ---------- Position encoding helpers ----------

// PositionEncoding returns the negotiated encoding for this client.
func (c *Client) PositionEncoding() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.positionEnc == "" {
		return "utf-16"
	}
	return c.positionEnc
}
