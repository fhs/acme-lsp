package proxy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fhs/go-lsp-internal/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

// Version is used to detect if acme-lsp and L are speaking the same protocol.
const Version = 1

// Server implements a subset of an LSP protocol server as defined by protocol.Server and
// some custom acme-lsp specific methods.
type Server interface {
	// Version returns the protocol version.
	Version(context.Context) (int, error)

	// WorkspaceFolders returns the workspace folders currently being managed by acme-lsp.
	// In LSP, this method is implemented by the client, but in our case acme-lsp is managing
	// the workspace folders, so this has to be implemented by the acme-lsp proxy server.
	WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error)

	// InitializeResult returns the initialize response from the LSP server.
	// This is useful for L command to get initialization results (e.g. server capabilities)
	// of an already initialized LSP server.
	InitializeResult(context.Context, *protocol.TextDocumentIdentifier) (*protocol.InitializeResult, error)

	// ExecuteCommandOnDocument is the same as ExecuteCommand, but
	// params contain the TextDocumentIdentifier of the original
	// CodeAction so that the server implemention can multiplex
	// ExecuteCommand request to the right server.
	ExecuteCommandOnDocument(context.Context, *ExecuteCommandOnDocumentParams) (interface{}, error)

	protocol.Server
	//DidChange(context.Context, *protocol.DidChangeTextDocumentParams) error
	//DidChangeWorkspaceFolders(context.Context, *protocol.DidChangeWorkspaceFoldersParams) error
	//Completion(context.Context, *protocol.CompletionParams) (*protocol.CompletionList, error)
	//Definition(context.Context, *protocol.DefinitionParams) ([]protocol.Location, error)
	//Formatting(context.Context, *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error)
	//CodeAction(context.Context, *protocol.CodeActionParams) ([]protocol.CodeAction, error)
	//Hover(context.Context, *protocol.HoverParams) (*protocol.Hover, error)
	//Implementation(context.Context, *protocol.ImplementationParams) ([]protocol.Location, error)
	//References(context.Context, *protocol.ReferenceParams) ([]protocol.Location, error)
	//Rename(context.Context, *protocol.RenameParams) (*protocol.WorkspaceEdit, error)
	//SignatureHelp(context.Context, *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error)
	//DocumentSymbol(context.Context, *protocol.DocumentSymbolParams) ([]interface{}, error)
	//TypeDefinition(context.Context, *protocol.TypeDefinitionParams) ([]protocol.Location, error)
}

func serverDispatch(ctx context.Context, server Server, conn *jsonrpc2.Conn, r *jsonrpc2.Request) (bool, error) {
	switch r.Method {
	case "acme-lsp/version": // req
		resp, err := server.Version(ctx)
		return true, reply(ctx, conn, r.ID, resp, err)

	case "acme-lsp/workspaceFolders": // req
		resp, err := server.WorkspaceFolders(ctx)
		return true, reply(ctx, conn, r.ID, resp, err)

	case "acme-lsp/initializeResult": // req
		var params protocol.TextDocumentIdentifier
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			return true, sendParseError(ctx, conn, r.ID, err)
		}
		resp, err := server.InitializeResult(ctx, &params)
		return true, reply(ctx, conn, r.ID, resp, err)

	case "acme-lsp/executeCommandOnDocument": // req
		var params ExecuteCommandOnDocumentParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			return true, sendParseError(ctx, conn, r.ID, err)
		}
		resp, err := server.ExecuteCommandOnDocument(ctx, &params)
		return true, reply(ctx, conn, r.ID, resp, err)

	default:
		return false, nil
	}
}

var _ Server = (*serverDispatcher)(nil)

// serverDispatcher extends a protocol.Server with our custom messages.
type serverDispatcher struct {
	*jsonrpc2.Conn
	protocol.Server
}

func (s *serverDispatcher) Version(ctx context.Context) (int, error) {
	var result int
	if err := s.Conn.Call(ctx, "acme-lsp/version", nil, &result); err != nil {
		return 0, err
	}
	return result, nil
}

func (s *serverDispatcher) WorkspaceFolders(ctx context.Context) ([]protocol.WorkspaceFolder, error) {
	var result []protocol.WorkspaceFolder
	if err := s.Conn.Call(ctx, "acme-lsp/workspaceFolders", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *serverDispatcher) InitializeResult(ctx context.Context, params *protocol.TextDocumentIdentifier) (*protocol.InitializeResult, error) {
	var result protocol.InitializeResult
	if err := s.Conn.Call(ctx, "acme-lsp/initializeResult", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *serverDispatcher) ExecuteCommandOnDocument(ctx context.Context, params *ExecuteCommandOnDocumentParams) (interface{}, error) {
	var result interface{}
	if err := s.Conn.Call(ctx, "acme-lsp/executeCommandOnDocument", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

var _ protocol.Server = (*NotImplementedServer)(nil)

// NotImplementedServer is a stub implementation of protocol.Server.
type NotImplementedServer struct{}

func (s *NotImplementedServer) Progress(context.Context, *protocol.ProgressParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) SetTrace(context.Context, *protocol.SetTraceParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) IncomingCalls(context.Context, *protocol.CallHierarchyIncomingCallsParams) ([]protocol.CallHierarchyIncomingCall, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) OutgoingCalls(context.Context, *protocol.CallHierarchyOutgoingCallsParams) ([]protocol.CallHierarchyOutgoingCall, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) ResolveCodeAction(context.Context, *protocol.CodeAction) (*protocol.CodeAction, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) ResolveCodeLens(context.Context, *protocol.CodeLens) (*protocol.CodeLens, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) ResolveCompletionItem(context.Context, *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) ResolveDocumentLink(context.Context, *protocol.DocumentLink) (*protocol.DocumentLink, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Exit(context.Context) error { return fmt.Errorf("not implemented") }
func (s *NotImplementedServer) Initialize(context.Context, *protocol.ParamInitialize) (*protocol.InitializeResult, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Initialized(context.Context, *protocol.InitializedParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Resolve(context.Context, *protocol.InlayHint) (*protocol.InlayHint, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidChangeNotebookDocument(context.Context, *protocol.DidChangeNotebookDocumentParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidCloseNotebookDocument(context.Context, *protocol.DidCloseNotebookDocumentParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidOpenNotebookDocument(context.Context, *protocol.DidOpenNotebookDocumentParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidSaveNotebookDocument(context.Context, *protocol.DidSaveNotebookDocumentParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Shutdown(context.Context) error { return fmt.Errorf("not implemented") }
func (s *NotImplementedServer) CodeAction(context.Context, *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) CodeLens(context.Context, *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) ColorPresentation(context.Context, *protocol.ColorPresentationParams) ([]protocol.ColorPresentation, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Completion(context.Context, *protocol.CompletionParams) (*protocol.Or_Result_textDocument_completion, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Declaration(context.Context, *protocol.DeclarationParams) (*protocol.Or_textDocument_declaration, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Definition(context.Context, *protocol.DefinitionParams) (*protocol.Or_Result_textDocument_definition, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Diagnostic(context.Context, *string) (*string, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidChange(context.Context, *protocol.DidChangeTextDocumentParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidClose(context.Context, *protocol.DidCloseTextDocumentParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidOpen(context.Context, *protocol.DidOpenTextDocumentParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidSave(context.Context, *protocol.DidSaveTextDocumentParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DocumentColor(context.Context, *protocol.DocumentColorParams) ([]protocol.ColorInformation, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DocumentHighlight(context.Context, *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DocumentLink(context.Context, *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DocumentSymbol(context.Context, *protocol.DocumentSymbolParams) ([]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) FoldingRange(context.Context, *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Formatting(context.Context, *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Hover(context.Context, *protocol.HoverParams) (*protocol.Hover, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Implementation(context.Context, *protocol.ImplementationParams) ([]protocol.Location, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) InlayHint(context.Context, *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) InlineCompletion(context.Context, *protocol.InlineCompletionParams) (*protocol.Or_Result_textDocument_inlineCompletion, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) InlineValue(context.Context, *protocol.InlineValueParams) ([]protocol.InlineValue, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) LinkedEditingRange(context.Context, *protocol.LinkedEditingRangeParams) (*protocol.LinkedEditingRanges, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Moniker(context.Context, *protocol.MonikerParams) ([]protocol.Moniker, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) OnTypeFormatting(context.Context, *protocol.DocumentOnTypeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) PrepareCallHierarchy(context.Context, *protocol.CallHierarchyPrepareParams) ([]protocol.CallHierarchyItem, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) PrepareRename(context.Context, *protocol.PrepareRenameParams) (*protocol.PrepareRenameResult, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) PrepareTypeHierarchy(context.Context, *protocol.TypeHierarchyPrepareParams) ([]protocol.TypeHierarchyItem, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) RangeFormatting(context.Context, *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) RangesFormatting(context.Context, *protocol.DocumentRangesFormattingParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) References(context.Context, *protocol.ReferenceParams) ([]protocol.Location, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Rename(context.Context, *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) SelectionRange(context.Context, *protocol.SelectionRangeParams) ([]protocol.SelectionRange, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) SemanticTokensFull(context.Context, *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) SemanticTokensFullDelta(context.Context, *protocol.SemanticTokensDeltaParams) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) SemanticTokensRange(context.Context, *protocol.SemanticTokensRangeParams) (*protocol.SemanticTokens, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) SignatureHelp(context.Context, *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) TypeDefinition(context.Context, *protocol.TypeDefinitionParams) ([]protocol.Location, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) WillSave(context.Context, *protocol.WillSaveTextDocumentParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) WillSaveWaitUntil(context.Context, *protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Subtypes(context.Context, *protocol.TypeHierarchySubtypesParams) ([]protocol.TypeHierarchyItem, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Supertypes(context.Context, *protocol.TypeHierarchySupertypesParams) ([]protocol.TypeHierarchyItem, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) WorkDoneProgressCancel(context.Context, *protocol.WorkDoneProgressCancelParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DiagnosticWorkspace(context.Context, *protocol.WorkspaceDiagnosticParams) (*protocol.WorkspaceDiagnosticReport, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidChangeConfiguration(context.Context, *protocol.DidChangeConfigurationParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidChangeWatchedFiles(context.Context, *protocol.DidChangeWatchedFilesParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidChangeWorkspaceFolders(context.Context, *protocol.DidChangeWorkspaceFoldersParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidCreateFiles(context.Context, *protocol.CreateFilesParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidDeleteFiles(context.Context, *protocol.DeleteFilesParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) DidRenameFiles(context.Context, *protocol.RenameFilesParams) error {
	return fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) ExecuteCommand(context.Context, *protocol.ExecuteCommandParams) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) Symbol(context.Context, *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) WillCreateFiles(context.Context, *protocol.CreateFilesParams) (*protocol.WorkspaceEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) WillDeleteFiles(context.Context, *protocol.DeleteFilesParams) (*protocol.WorkspaceEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) WillRenameFiles(context.Context, *protocol.RenameFilesParams) (*protocol.WorkspaceEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) ResolveWorkspaceSymbol(context.Context, *protocol.WorkspaceSymbol) (*protocol.WorkspaceSymbol, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *NotImplementedServer) NonstandardRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
