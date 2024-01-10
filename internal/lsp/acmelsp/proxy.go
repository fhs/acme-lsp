package acmelsp

import (
	"context"
	"fmt"
	"log"

	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/fhs/acme-lsp/internal/p9service"
	"github.com/fhs/go-lsp-internal/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

type proxyServer struct {
	ss *ServerSet // client connections to upstream LSP server (e.g. gopls)
	fm *FileManager
	proxy.NotImplementedServer
}

func (s *proxyServer) Version(ctx context.Context) (int, error) {
	return proxy.Version, nil
}

func (s *proxyServer) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return s.ss.Workspaces(), nil
}

func (s *proxyServer) InitializeResult(ctx context.Context, params *protocol.TextDocumentIdentifier) (*protocol.InitializeResult, error) {
	srv, err := serverForURI(s.ss, params.URI)
	if err != nil {
		return nil, fmt.Errorf("InitializeResult: %v", err)
	}
	return srv.Client.InitializeResult(ctx, params)
}

func (s *proxyServer) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	srv, err := serverForURI(s.ss, params.TextDocument.URI)
	if err != nil {
		return fmt.Errorf("DidChange: %v", err)
	}
	return srv.Client.DidChange(ctx, params)
}

func (s *proxyServer) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) error {
	return s.ss.DidChangeWorkspaceFolders(ctx, params.Event.Added, params.Event.Removed)
}

func (s *proxyServer) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.Or_Result_textDocument_completion, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("Completion: %v", err)
	}
	return srv.Client.Completion(ctx, params)
}

func (s *proxyServer) Definition(ctx context.Context, params *protocol.DefinitionParams) (*protocol.Or_Result_textDocument_definition, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("Definition: %v", err)
	}
	return srv.Client.Definition(ctx, params)
}

func (s *proxyServer) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	srv, err := serverForURI(s.ss, params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("Formatting: %v", err)
	}
	return srv.Client.Formatting(ctx, params)
}

func (s *proxyServer) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	srv, err := serverForURI(s.ss, params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("CodeAction: %v", err)
	}
	return srv.Client.CodeAction(ctx, params)
}

func (s *proxyServer) ExecuteCommandOnDocument(ctx context.Context, params *proxy.ExecuteCommandOnDocumentParams) (interface{}, error) {
	srv, err := serverForURI(s.ss, params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("ExecuteCommandOnDocument: %v", err)
	}
	return srv.Client.ExecuteCommand(ctx, &params.ExecuteCommandParams)
}

func (s *proxyServer) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("Hover: %v", err)
	}
	return srv.Client.Hover(ctx, params)
}

func (s *proxyServer) Implementation(ctx context.Context, params *protocol.ImplementationParams) ([]protocol.Location, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("Implementation: %v", err)
	}
	return srv.Client.Implementation(ctx, params)
}

func (s *proxyServer) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("References: %v", err)
	}
	return srv.Client.References(ctx, params)
}

func (s *proxyServer) Rename(ctx context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	srv, err := serverForURI(s.ss, params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("Rename: %v", err)
	}
	return srv.Client.Rename(ctx, params)
}

func (s *proxyServer) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("SignatureHelp: %v", err)
	}
	return srv.Client.SignatureHelp(ctx, params)
}

func (s *proxyServer) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]interface{}, error) {
	srv, err := serverForURI(s.ss, params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("DocumentSymbol: %v", err)
	}
	return srv.Client.DocumentSymbol(ctx, params)
}

func (s *proxyServer) TypeDefinition(ctx context.Context, params *protocol.TypeDefinitionParams) ([]protocol.Location, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("TypeDefinition: %v", err)
	}
	return srv.Client.TypeDefinition(ctx, params)
}

func serverForURI(ss *ServerSet, uri protocol.DocumentURI) (*Server, error) {
	filename := text.ToPath(uri)
	srv, found, err := ss.StartForFile(filename)
	if !found {
		return nil, fmt.Errorf("unknown language server for URI %q", uri)
	}
	if err != nil {
		return nil, fmt.Errorf("cound not start language server: %v", err)
	}
	return srv, nil
}

func ListenAndServeProxy(ctx context.Context, cfg *config.Config, ss *ServerSet, fm *FileManager) error {
	ln, err := p9service.Listen(ctx, cfg.ProxyNetwork, cfg.ProxyAddress)
	if err != nil {
		return err
	}
	// The context doesn't affect Accept below,
	// so roll our own cancellation/timeout.
	// See https://github.com/golang/go/issues/28120#issuecomment-428978461
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		stream := jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{})
		handler := proxy.NewServerHandler(&proxyServer{
			ss: ss,
			fm: fm,
		})
		var opts []jsonrpc2.ConnOpt
		if cfg.RPCTrace {
			opts = append(opts, lsp.LogMessages(log.Default()))

		}
		rpc := jsonrpc2.NewConn(ctx, stream, handler, opts...)
		go func() {
			<-rpc.DisconnectNotify()
			if cfg.Verbose {
				log.Printf("proxy: jsonrpc2 connection disconnected\n")
			}
		}()
	}
}
