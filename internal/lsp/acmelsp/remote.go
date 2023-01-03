package acmelsp

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
	"github.com/fhs/acme-lsp/internal/lsp/text"
)

// RemoteCmd executes LSP commands in an acme window using the proxy server.
type RemoteCmd struct {
	server proxy.Server
	winid  int
	offset int
	Stdout io.Writer
	Stderr io.Writer
}

func NewRemoteCmd(server proxy.Server, winid int, offset int) *RemoteCmd {
	return &RemoteCmd{
		server: server,
		winid:  winid,
		offset: offset,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func (rc *RemoteCmd) getPosition() (pos *protocol.TextDocumentPositionParams, filename string, err error) {
	w, err := acmeutil.OpenWin(rc.winid)
	if err != nil {
		return nil, "", fmt.Errorf("failed to to open window %v: %v", rc.winid, err)
	}
	defer w.CloseFiles()
	if rc.offset != -1 {
		return text.PositionQ0(w, rc.offset)
	}
	return text.Position(w)
}

func (rc *RemoteCmd) DidChange(ctx context.Context) error {
	w, err := acmeutil.OpenWin(rc.winid)
	if err != nil {
		return fmt.Errorf("failed to to open window %v: %v", rc.winid, err)
	}
	defer w.CloseFiles()

	uri, _, err := text.DocumentURI(w)
	if err != nil {
		return err
	}
	body, err := w.ReadAll("body")
	if err != nil {
		return err
	}

	return rc.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: uri,
			},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Text: string(body),
			},
		},
	})
}

func (rc *RemoteCmd) Completion(ctx context.Context, edit bool) error {
	w, err := acmeutil.OpenWin(rc.winid)
	if err != nil {
		return err
	}
	defer w.CloseFiles()

	pos, _, err := text.Position(w)
	if err != nil {
		return err
	}
	result, err := rc.server.Completion(ctx, &protocol.CompletionParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	if edit && len(result.Items) == 1 {
		textEdit := result.Items[0].TextEdit
		if textEdit == nil {
			// TODO(fhs): Use insertText or label instead.
			return fmt.Errorf("nil TextEdit in completion item")
		}
		if err := text.Edit(w, []protocol.TextEdit{*textEdit}); err != nil {
			return fmt.Errorf("failed to apply completion edit: %v", err)
		}
		return nil
	}
	if len(result.Items) == 0 {
		fmt.Fprintf(rc.Stderr, "no completion\n")
	}
	for _, item := range result.Items {
		fmt.Fprintf(rc.Stdout, "%v %v\n", item.Label, item.Detail)
	}
	return nil
}

func (rc *RemoteCmd) Definition(ctx context.Context, print bool) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return fmt.Errorf("failed to get position: %v", err)
	}
	locations, err := rc.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return fmt.Errorf("bad server response: %v", err)
	}
	if print {
		return PrintLocations(rc.Stdout, locations)
	}
	return PlumbLocations(locations)
}

func (rc *RemoteCmd) OrganizeImportsAndFormat(ctx context.Context) error {
	win, err := acmeutil.OpenWin(rc.winid)
	if err != nil {
		return err
	}
	defer win.CloseFiles()

	uri, _, err := text.DocumentURI(win)
	if err != nil {
		return err
	}

	doc := &protocol.TextDocumentIdentifier{
		URI: uri,
	}
	return CodeActionAndFormat(ctx, rc.server, doc, win, []protocol.CodeActionKind{
		protocol.SourceOrganizeImports,
	})
}

func (rc *RemoteCmd) Hover(ctx context.Context) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	hov, err := rc.server.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(rc.Stdout, "%v\n", hov.Contents.Value)
	return nil
}

func (rc *RemoteCmd) Implementation(ctx context.Context, print bool) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	loc, err := rc.server.Implementation(ctx, &protocol.ImplementationParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	if len(loc) == 0 {
		fmt.Fprintf(rc.Stderr, "No implementations found.\n")
		return nil
	}
	return PrintLocations(rc.Stdout, loc)
}

func (rc *RemoteCmd) References(ctx context.Context) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	loc, err := rc.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: *pos,
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	})
	if err != nil {
		return err
	}
	if len(loc) == 0 {
		fmt.Fprintf(rc.Stderr, "No references found.\n")
		return nil
	}
	return PrintLocations(rc.Stdout, loc)
}

// Rename renames the identifier at cursor position to newname.
func (rc *RemoteCmd) Rename(ctx context.Context, newname string) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	we, err := rc.server.Rename(ctx, &protocol.RenameParams{
		TextDocument: pos.TextDocument,
		Position:     pos.Position,
		NewName:      newname,
	})
	if err != nil {
		return err
	}
	return editWorkspace(we)
}

func (rc *RemoteCmd) SignatureHelp(ctx context.Context) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	sh, err := rc.server.SignatureHelp(ctx, &protocol.SignatureHelpParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	for _, sig := range sh.Signatures {
		fmt.Fprintf(rc.Stdout, "%v\n", sig.Label)
		fmt.Fprintf(rc.Stdout, "%v\n", sig.Documentation)
	}
	return nil
}

func (rc *RemoteCmd) DocumentSymbol(ctx context.Context) error {
	win, err := acmeutil.OpenWin(rc.winid)
	if err != nil {
		return err
	}
	defer win.CloseFiles()

	uri, _, err := text.DocumentURI(win)
	if err != nil {
		return err
	}

	// TODO(fhs): DocumentSymbol request can return either a
	// []DocumentSymbol (hierarchical) or []SymbolInformation (flat).
	// We only handle the hierarchical type below.

	// TODO(fhs): Make use of DocumentSymbol.Range to optionally filter out
	// symbols that aren't within current cursor position?

	syms, err := rc.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	})
	if err != nil {
		return err
	}
	if len(syms) == 0 {
		fmt.Fprintf(rc.Stderr, "No symbols found.\n")
		return nil
	}
	walkDocumentSymbols(syms, 0, func(s *protocol.DocumentSymbol, depth int) {
		loc := &protocol.Location{
			URI:   uri,
			Range: s.SelectionRange,
		}
		indent := strings.Repeat(" ", depth)
		fmt.Fprintf(rc.Stdout, "%v%v %v %v\n", indent, s.Kind, s.Name, s.Detail)
		fmt.Fprintf(rc.Stdout, "%v %v\n", indent, lsp.LocationLink(loc))
	})
	return nil
}

func (rc *RemoteCmd) TypeDefinition(ctx context.Context, print bool) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	locations, err := rc.server.TypeDefinition(ctx, &protocol.TypeDefinitionParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	if print {
		return PrintLocations(rc.Stdout, locations)
	}
	return PlumbLocations(locations)
}

func walkDocumentSymbols(syms []protocol.DocumentSymbol, depth int, f func(s *protocol.DocumentSymbol, depth int)) {
	for _, s := range syms {
		f(&s, depth)
		walkDocumentSymbols(s.Children, depth+1, f)
	}
}
