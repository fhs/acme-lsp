// Package acmelsp implements the core of acme-lsp commands.
package acmelsp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/fhs/9fans-go/plan9"
	"github.com/fhs/9fans-go/plumb"
	"github.com/fhs/acme-lsp/internal/acme"
	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
	"github.com/fhs/acme-lsp/internal/lsp/text"
)

func CurrentWindowRemoteCmd(ss *ServerSet, fm *FileManager) (*RemoteCmd, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse $winid: %v", err)
	}
	return WindowRemoteCmd(ss, fm, id)
}

func WindowRemoteCmd(ss *ServerSet, fm *FileManager, winid int) (*RemoteCmd, error) {
	w, err := acmeutil.OpenWin(winid)
	if err != nil {
		return nil, fmt.Errorf("failed to to open window %v: %v", winid, err)
	}
	defer w.CloseFiles()

	_, fname, err := text.DocumentURI(w)
	if err != nil {
		return nil, fmt.Errorf("failed to get text position: %v", err)
	}
	srv, found, err := ss.StartForFile(fname)
	if err != nil {
		return nil, fmt.Errorf("cound not start language server: %v", err)
	}
	if !found {
		return nil, fmt.Errorf("no language server for filename %q", fname)
	}

	// In case the window has unsaved changes (it's dirty),
	// send changes to LSP server.
	if err = fm.didChange(winid, fname); err != nil {
		return nil, fmt.Errorf("DidChange failed: %v", err)
	}

	return NewRemoteCmd(srv.Client, winid, -1), nil
}

func getLine(p string, l int) string {
	file, err := os.Open(p)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentLine := 0
	for scanner.Scan() {
		currentLine++
		if currentLine == l {
			return scanner.Text()
		}
	}

	return ""
}

func PrintLocations(w io.Writer, loc []protocol.Location) error {
	sort.Slice(loc, func(i, j int) bool {
		a := loc[i]
		b := loc[j]
		n := strings.Compare(string(a.URI), string(b.URI))
		if n == 0 {
			m := a.Range.Start.Line - b.Range.Start.Line
			if m == 0 {
				return a.Range.Start.Character < b.Range.Start.Character
			}
			return m < 0
		}
		return n < 0
	})
	for _, l := range loc {
		fmt.Fprintf(w, "%v:%s\n", lsp.LocationLink(&l), getLine(text.ToPath(l.URI), int(l.Range.Start.Line+1)))
	}
	return nil
}

// PlumbLocations sends the locations to the plumber.
func PlumbLocations(locations []protocol.Location) error {
	p, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		return fmt.Errorf("failed to open plumber: %v", err)
	}
	defer p.Close()
	for _, loc := range locations {
		err := plumbLocation(&loc).Send(p)
		if err != nil {
			return fmt.Errorf("failed to plumb location: %v", err)
		}
	}
	return nil
}

func plumbLocation(loc *protocol.Location) *plumb.Message {
	// LSP uses zero-based offsets.
	// Place the cursor *before* the location range.
	pos := loc.Range.Start
	attr := &plumb.Attribute{
		Name:  "addr",
		Value: fmt.Sprintf("%v-#0+#%v", pos.Line+1, pos.Character),
	}
	return &plumb.Message{
		Src:  "acme-lsp",
		Dst:  "edit",
		Dir:  "/",
		Type: "text",
		Attr: attr,
		Data: []byte(text.ToPath(loc.URI)),
	}
}

type FormatServer interface {
	InitializeResult(context.Context, *protocol.TextDocumentIdentifier) (*protocol.InitializeResult, error)
	DidChange(context.Context, *protocol.DidChangeTextDocumentParams) error
	Formatting(context.Context, *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error)
	CodeAction(context.Context, *protocol.CodeActionParams) ([]protocol.CodeAction, error)
	ExecuteCommandOnDocument(context.Context, *proxy.ExecuteCommandOnDocumentParams) (interface{}, error)
}

// CodeActionAndFormat runs the given code actions and then formats the file f.
func CodeActionAndFormat(ctx context.Context, server FormatServer, doc *protocol.TextDocumentIdentifier, f text.File, actions []protocol.CodeActionKind) error {
	initres, err := server.InitializeResult(ctx, doc)
	if err != nil {
		return err
	}

	actions = lsp.CompatibleCodeActions(&initres.Capabilities, actions)
	if len(actions) > 0 {
		actions, err := server.CodeAction(ctx, &protocol.CodeActionParams{
			TextDocument: *doc,
			Range:        protocol.Range{},
			Context: protocol.CodeActionContext{
				Diagnostics: nil,
				Only:        actions,
			},
		})
		if err != nil {
			return err
		}
		for _, a := range actions {
			if a.Edit != nil {
				err := editWorkspace(a.Edit)
				if err != nil {
					return err
				}
			}
			if a.Command != nil {
				_, err := server.ExecuteCommandOnDocument(ctx, &proxy.ExecuteCommandOnDocumentParams{
					TextDocument: *doc,
					ExecuteCommandParams: protocol.ExecuteCommandParams{
						Command:   a.Command.Command,
						Arguments: a.Command.Arguments,
					},
				})
				if err != nil {
					return err
				}
			}
		}
		if len(actions) > 0 {
			// Our file may or may not be among the workspace edits for import fixes.
			// We assume it is among the edits.
			// TODO(fhs): Skip if our file didn't have import changes.
			rd, err := f.Reader()
			if err != nil {
				return err
			}
			b, err := ioutil.ReadAll(rd)
			if err != nil {
				return err
			}
			server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: *doc,
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{
					{
						Text: string(b),
					},
				},
			})
			if err != nil {
				return err
			}
		}
	}
	edits, err := server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: *doc,
	})
	if err != nil {
		return err
	}
	if err := text.Edit(f, edits); err != nil {
		return fmt.Errorf("failed to apply edits: %v", err)
	}
	return nil
}

func editWorkspace(we *protocol.WorkspaceEdit) error {
	if we == nil {
		return nil // no changes to apply
	}
	if we.Changes == nil && we.DocumentChanges != nil {
		// gopls version >= 0.3.1 sends versioned document edits
		// for organizeImports code action even when we don't
		// support it. Convert versioned edits to non-versioned.
		changes := make(map[string][]protocol.TextEdit)
		for _, dc := range we.DocumentChanges {
			changes[dc.TextDocument.TextDocumentIdentifier.URI] = dc.Edits
		}
		we.Changes = &changes
	}
	if we.Changes == nil {
		return nil // no changes to apply
	}

	wins, err := acme.Windows()
	if err != nil {
		return fmt.Errorf("failed to read list of acme index: %v", err)
	}
	winid := make(map[string]int, len(wins))
	for _, info := range wins {
		winid[info.Name] = info.ID
	}

	for uri := range *we.Changes {
		fname := text.ToPath(uri)
		if _, ok := winid[fname]; !ok {
			return fmt.Errorf("%v: not open in acme", fname)
		}
	}
	for uri, edits := range *we.Changes {
		fname := text.ToPath(uri)
		id := winid[fname]
		w, err := acmeutil.OpenWin(id)
		if err != nil {
			return fmt.Errorf("failed to open window %v: %v", id, err)
		}
		if err := text.Edit(w, edits); err != nil {
			return fmt.Errorf("failed to apply edits to window %v: %v", id, err)
		}
		w.CloseFiles()
	}
	return nil
}
