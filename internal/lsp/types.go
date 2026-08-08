package lsp

import (
	"fmt"
)

type ServerConfig struct {
	Name                string
	Command             string
	Args                []string
	ExtensionToLanguage map[string]string
}

type PluginServer struct {
	Name   string
	Config ServerConfig
}

// Capabilities reported by the server's initialize result.
type ServerCapabilities struct {
	PositionEncoding string
	Hover            bool
	Definition       bool
	References       bool
	DocumentSymbol   bool
	Rename           bool
	Diagnostics      bool
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// InitializeParams mirrors the LSP initialize request payload we send.
type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootPath     string             `json:"rootPath,omitempty"`
	RootURI      string             `json:"rootUri,omitempty"`
	Trace        string             `json:"trace,omitempty"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

type ClientCapabilities struct {
	PositionEncodings []string                       `json:"positionEncodings,omitempty"`
	TextDocument      TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace         WorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Synchronization    SynchronizationCapabilities    `json:"synchronization,omitempty"`
	Definition         GenericCapability              `json:"definition,omitempty"`
	References         GenericCapability              `json:"references,omitempty"`
	DocumentSymbol     GenericCapability              `json:"documentSymbol,omitempty"`
	Rename             GenericCapability              `json:"rename,omitempty"`
	PublishDiagnostics PublishDiagnosticsCapabilities `json:"publishDiagnostics,omitempty"`
}

type SynchronizationCapabilities struct {
	DidOpen           bool `json:"didOpen,omitempty"`
	DidChange         bool `json:"didChange,omitempty"`
	WillSaveWaitUntil bool `json:"willSaveWaitUntil,omitempty"`
	DidClose          bool `json:"didClose,omitempty"`
}

type GenericCapability struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type PublishDiagnosticsCapabilities struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
}

type WorkspaceClientCapabilities struct {
	WorkspaceFolders bool             `json:"workspaceFolders,omitempty"`
	ApplyEdit        VerifyCapability `json:"applyEdit,omitempty"`
}

type VerifyCapability struct {
	ValueUpdated bool `json:"valueUpdated,omitempty"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

// Diagnostic is a single LSP diagnostic entry.
type Diagnostic struct {
	Range    LSPRange `json:"range"`
	Severity int      `json:"severity,omitempty"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func (c ServerConfig) String() string {
	return fmt.Sprintf("%s (%s)", c.Name, c.Command)
}
