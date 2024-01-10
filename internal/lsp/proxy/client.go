package proxy

import (
	"context"
	"fmt"

	"github.com/fhs/go-lsp-internal/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

const Debug = false

type Client interface {
	protocol.Client
}

func clientDispatch(ctx context.Context, client Client, conn *jsonrpc2.Conn, r *jsonrpc2.Request) (bool, error) {
	return false, nil
}

var _ Client = (*clientDispatcher)(nil)

type clientDispatcher struct {
	*jsonrpc2.Conn
	protocol.Client
}

var _ protocol.Client = (*NotImplementedClient)(nil)

type NotImplementedClient struct{}

// $/logTrace
func (c *NotImplementedClient) LogTrace(context.Context, *protocol.LogTraceParams) error {
	return fmt.Errorf("$/logTrace not implemented")
}

// $/progress
func (c *NotImplementedClient) Progress(context.Context, *protocol.ProgressParams) error {
	return fmt.Errorf("$/progress not implemented")
}

func (c *NotImplementedClient) ShowMessage(context.Context, *protocol.ShowMessageParams) error {
	return fmt.Errorf("ShowMessage not implemented")
}

func (c *NotImplementedClient) LogMessage(ctx context.Context, params *protocol.LogMessageParams) error {
	return fmt.Errorf("LogMessage not implemented")
}

// window/showDocument
func (c *NotImplementedClient) ShowDocument(context.Context, *protocol.ShowDocumentParams) (*protocol.ShowDocumentResult, error) {
	return nil, fmt.Errorf("window/showDocument not implemented")
}

func (c *NotImplementedClient) Event(context.Context, *interface{}) error {
	return fmt.Errorf("Event not implemented")
}

func (c *NotImplementedClient) PublishDiagnostics(context.Context, *protocol.PublishDiagnosticsParams) error {
	return fmt.Errorf("PublishDiagnostics not implemented")
}

func (c *NotImplementedClient) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return nil, fmt.Errorf("WorkspaceFolders not implemented")
}

func (c *NotImplementedClient) Configuration(context.Context, *protocol.ParamConfiguration) ([]interface{}, error) {
	return nil, fmt.Errorf("Configuration not implemented")
}

func (c *NotImplementedClient) RegisterCapability(context.Context, *protocol.RegistrationParams) error {
	return fmt.Errorf("RegisterCapability not implemented")
}

func (c *NotImplementedClient) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	return fmt.Errorf("UnregisterCapability not implemented")
}

func (c *NotImplementedClient) ShowMessageRequest(context.Context, *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	return nil, fmt.Errorf("ShowMessageRequest not implemented")
}

// window/workDoneProgress/create
func (c *NotImplementedClient) WorkDoneProgressCreate(context.Context, *protocol.WorkDoneProgressCreateParams) error {
	return fmt.Errorf("window/workDoneProgress/create not implemented")
}

func (c *NotImplementedClient) ApplyEdit(context.Context, *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResult, error) {
	return nil, fmt.Errorf("ApplyEdit not implemented")
}

// workspace/codeLens/refresh
func (c *NotImplementedClient) CodeLensRefresh(context.Context) error {
	return fmt.Errorf("workspace/codeLens/refresh not implemented")
}

// workspace/diagnostic/refresh
func (c *NotImplementedClient) DiagnosticRefresh(context.Context) error {
	return fmt.Errorf("workspace/diagnostic/refresh not implemented")
}

// workspace/foldingRange/refresh
func (c *NotImplementedClient) FoldingRangeRefresh(context.Context) error {
	return fmt.Errorf("workspace/foldingRange/refresh not implemented")
}

// workspace/inlayHint/refresh
func (c *NotImplementedClient) InlayHintRefresh(context.Context) error {
	return fmt.Errorf("workspace/inlayHint/refresh not implemented")
}

// workspace/inlineValue/refresh
func (c *NotImplementedClient) InlineValueRefresh(context.Context) error {
	return fmt.Errorf("workspace/inlineValue/refresh not implemented")
}

// workspace/semanticTokens/refresh
func (c *NotImplementedClient) SemanticTokensRefresh(context.Context) error {
	return fmt.Errorf("workspace/semanticTokens/refresh not implemented")
}
