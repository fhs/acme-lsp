package acmelsp

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/fhs/go-lsp-internal/lsp/protocol"
)

const goSource = `package main // import "example.com/test"

import "fmt"

func main() {
	fmt.Println("Hello, 世界")
}
`

const goSourceUnfmt = `package main // import "example.com/test"

import "fmt"

func main( ){
fmt . Println	( "Hello, 世界" )
}
`

const goMod = `module github.com/fhs/acme-lsp/internal/lsp/acmelsp/client_test
`

func testGoModule(t *testing.T, server string, src string, f func(t *testing.T, c *Client, uri protocol.DocumentURI)) {
	serverArgs := map[string][]string{
		"gopls": {"gopls"},
	}

	// Create the module
	dir, err := ioutil.TempDir("", "examplemod")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	gofile := filepath.Join(dir, "main.go")
	if err := ioutil.WriteFile(gofile, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	modfile := filepath.Join(dir, "go.mod")
	if err := ioutil.WriteFile(modfile, []byte(goMod), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Start the server
	args, ok := serverArgs[server]
	if !ok {
		t.Fatalf("unknown server %q", server)
	}
	cs := &config.Server{
		Command: args,
	}
	srv, err := execServer(cs, &ClientConfig{
		Server:        &config.Server{},
		RootDirectory: dir,
		DiagWriter:    &mockDiagosticsWriter{ioutil.Discard},
		Workspaces:    nil,
	}, false)
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	ctx := context.Background()
	err = lsp.DidOpen(ctx, srv.Client, gofile, "go", []byte(src))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}
	defer func() {
		err := lsp.DidClose(ctx, srv.Client, gofile)
		if err != nil {
			t.Fatalf("DidClose failed: %v", err)
		}
		srv.Client.Close()
	}()

	t.Run(server, func(t *testing.T) {
		f(t, srv.Client, text.ToURI(gofile))
	})
}

func TestGoFormat(t *testing.T) {
	ctx := context.Background()

	for _, server := range []string{
		"gopls",
	} {
		testGoModule(t, server, goSourceUnfmt, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
			edits, err := c.Formatting(ctx, &protocol.DocumentFormattingParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
			})
			if err != nil {
				t.Fatalf("Format failed: %v", err)
			}
			f := BytesFile([]byte(goSourceUnfmt))
			err = text.Edit(&f, edits)
			if err != nil {
				t.Fatalf("failed to apply edits: %v", err)
			}
			if got := string(f); got != goSource {
				t.Errorf("bad format output:\n%s\nexpected:\n%s", got, goSource)
			}
		})
	}
}

func TestGoHover(t *testing.T) {
	ctx := context.Background()

	for _, srv := range []struct {
		name string
		want string
	}{
		{"gopls", "```go\nfunc fmt.Println(a ...any) (n int, err error)\n```\n\nPrintln formats using the default formats for its operands and writes to standard output. Spaces are always added between operands and a newline is appended. It returns the number of bytes written and any write error encountered.\n\n\n[`fmt.Println` on pkg.go.dev](https://pkg.go.dev/fmt#Println)"},
	} {
		testGoModule(t, srv.name, goSource, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
			pos := &protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
				Position: protocol.Position{
					Line:      5,
					Character: 10,
				},
			}
			hov, err := c.Hover(ctx, &protocol.HoverParams{
				TextDocumentPositionParams: *pos,
			})
			if err != nil {
				t.Fatalf("Hover failed: %v", err)
			}
			got := hov.Contents.Value
			// Instead of doing an exact match, we ignore extra markups
			// from markdown (if there are any).
			if !strings.Contains(got, srv.want) {
				t.Errorf("hover result is %q; expected %q", got, srv.want)
			}
		})
	}
}

func TestGoDefinition(t *testing.T) {
	src := `package main // import "example.com/test"

import "fmt"

func hello() string { return "Hello" }

func main() {
	fmt.Printf("%v\n", hello())
}
`

	for _, srv := range []string{
		"gopls",
	} {
		testGoModule(t, srv, src, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
			params := &protocol.DefinitionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{
						URI: uri,
					},
					Position: protocol.Position{
						Line:      7,
						Character: 22,
					},
				},
			}
			got, err := c.Definition(context.Background(), params)
			if err != nil {
				t.Fatalf("Definition failed: %v", err)
			}
			want := &protocol.Or_Result_textDocument_definition{
				Value: protocol.Definition{
					Value: []protocol.Location{
						{
							URI: uri,
							Range: protocol.Range{
								Start: protocol.Position{
									Line:      4,
									Character: 5,
								},
								End: protocol.Position{
									Line:      4,
									Character: 10,
								},
							},
						},
					},
				},
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("definition result is %v; expected %v", got, want)
			}
		})
	}
}

func TestGoTypeDefinition(t *testing.T) {
	ctx := context.Background()
	src := `package main // import "example.com/test"

import "fmt"

type T string

func main() {
	foo := T("hello")
	fmt.Printf("%v\n", foo)
}
`

	for _, srv := range []string{
		"gopls",
	} {
		testGoModule(t, srv, src, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
			pos := &protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
				Position: protocol.Position{
					Line:      7,
					Character: 2,
				},
			}
			got, err := c.TypeDefinition(ctx, &protocol.TypeDefinitionParams{
				TextDocumentPositionParams: *pos,
			})
			if err != nil {
				t.Fatalf("TypeDefinition failed: %v", err)
			}
			want := []protocol.Location{
				{
					URI: uri,
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      4,
							Character: 5,
						},
						End: protocol.Position{
							Line:      4,
							Character: 6,
						},
					},
				},
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("type definition result is %v; expected %v", got, want)
			}
		})
	}
}

func TestGoDiagnostics(t *testing.T) {
	src := `package main // import "example.com/test"

func main() {
	var s string
}
`
	dir, err := ioutil.TempDir("", "examplemod")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	gofile := filepath.Join(dir, "main.go")
	if err := ioutil.WriteFile(gofile, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	modfile := filepath.Join(dir, "go.mod")
	if err := ioutil.WriteFile(modfile, []byte(goMod), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ch := make(chan *protocol.Diagnostic)
	cs := &config.Server{
		Command: []string{"gopls"},
	}
	srv, err := execServer(cs, &ClientConfig{
		Server:        &config.Server{},
		RootDirectory: dir,
		DiagWriter:    &chanDiagosticsWriter{ch},
		Workspaces:    nil,
	}, false)
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	ctx := context.Background()
	err = lsp.DidOpen(ctx, srv.Client, gofile, "go", []byte(src))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}

	diag := <-ch
	pat := regexp.MustCompile("^s declared (and|but) not used$")
	if !pat.MatchString(diag.Message) {
		t.Errorf("diagnostics message is %q does not match %q", diag.Message, pat)
	}

	err = lsp.DidClose(ctx, srv.Client, gofile)
	if err != nil {
		t.Fatalf("DidClose failed: %v", err)
	}
	srv.Client.Close()
}

const pySource = `#!/usr/bin/env python

import math


def main():
    print(math.sqrt(42))


if __name__ == '__main__':
    main()
`

const pySourceUnfmt = `#!/usr/bin/env python

import math

def main( ):
    print( math.sqrt ( 42 ) )

if __name__=='__main__':
    main( )
`

func testPython(t *testing.T, src string, f func(t *testing.T, c *Client, uri protocol.DocumentURI)) {
	dir, err := ioutil.TempDir("", "lspexample")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	pyfile := filepath.Join(dir, "main.py")
	if err := ioutil.WriteFile(pyfile, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Start the server
	cs := &config.Server{
		Command: []string{"pyls"},
	}
	srv, err := execServer(cs, &ClientConfig{
		Server:        &config.Server{},
		RootDirectory: dir,
		DiagWriter:    &mockDiagosticsWriter{ioutil.Discard},
		Workspaces:    nil,
	}, false)
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	ctx := context.Background()
	err = lsp.DidOpen(ctx, srv.Client, pyfile, "python", []byte(src))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}
	defer func() {
		err := lsp.DidClose(ctx, srv.Client, pyfile)
		if err != nil {
			t.Fatalf("DidClose failed: %v", err)
		}
		srv.Client.Close()
	}()

	f(t, srv.Client, text.ToURI(pyfile))
}

func TestPythonHover(t *testing.T) {
	t.Skip("hangs")
	ctx := context.Background()

	testPython(t, pySource, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
		pos := &protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Position: protocol.Position{
				Line:      6,
				Character: 16,
			},
		}
		hov, err := c.Hover(ctx, &protocol.HoverParams{
			TextDocumentPositionParams: *pos,
		})
		if err != nil {
			t.Fatalf("Hover failed: %v", err)
		}
		got := hov.Contents.Value
		want := "Return the square root of x."
		// May not be an exact match.
		// Perhaps depending on if it's Python 2 or 3?
		if !strings.Contains(got, want) {
			t.Errorf("hover result is %q does not contain %q", got, want)
		}
	})
}

func TestPythonFormat(t *testing.T) {
	t.Skip("hangs")
	ctx := context.Background()

	testPython(t, pySourceUnfmt, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
		edits, err := c.Formatting(ctx, &protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
		})
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}
		f := BytesFile([]byte(pySourceUnfmt))
		err = text.Edit(&f, edits)
		if err != nil {
			t.Fatalf("failed to apply edits: %v", err)
		}
		if got := string(f); got != pySource {
			t.Errorf("bad format output:\n%s\nexpected:\n%s", got, pySource)
		}
	})
}

func TestPythonDefinition(t *testing.T) {
	t.Skip("hangs")
	if runtime.GOOS == "windows" {
		t.Skip("TODO: failing on windows due to file path issues")
	}

	src := `def main():
    pass

if __name__ == '__main__':
    main()
`

	testPython(t, src, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
		params := &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
				Position: protocol.Position{
					Line:      4,
					Character: 6,
				},
			},
		}
		got, err := c.Definition(context.Background(), params)
		if err != nil {
			t.Fatalf("Definition failed: %v", err)
		}
		want := []protocol.Location{
			{
				URI: uri,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      0,
						Character: 4,
					},
					End: protocol.Position{
						Line:      0,
						Character: 8,
					},
				},
			},
		}
		if !reflect.DeepEqual(want, got) {
			t.Errorf("definition result is %v; expected %v", got, want)
		}
	})
}

func TestPythonTypeDefinition(t *testing.T) {
	t.Logf("pyls doesn't implement LSP textDocument/typeDefinition")
}

func TestFileLanguage(t *testing.T) {
	for _, tc := range []struct {
		name, lang string
	}{
		{"/home/gopher/hello.py", "python"},
		{"/home/gopher/hello.go", "go"},
		{"/home/gopher/go.mod", "go.mod"},
		{"/home/gopher/go.sum", "go.sum"},
		{"/home/gopher/.config/acme-lsp/config.toml", "toml"},
	} {
		lang := lsp.DetectLanguage(tc.name)
		if lang != tc.lang {
			t.Errorf("language ID of %q is %q; expected %q", tc.name, lang, tc.lang)
		}
	}
}

func TestLocationLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: failing on windows due to file path issues")
	}

	l := &protocol.Location{
		URI: protocol.DocumentURI("file:///home/gopher/mod1/main.go"),
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      13,
				Character: 9,
			},
			End: protocol.Position{
				Line:      15,
				Character: 7,
			},
		},
	}
	got := lsp.LocationLink(l)
	want := "/home/gopher/mod1/main.go:14:10-16:8"
	if got != want {
		t.Errorf("LocationLink(%v) returned %q; want %q", l, got, want)
	}
}

type chanDiagosticsWriter struct {
	ch chan *protocol.Diagnostic
}

func (dw *chanDiagosticsWriter) WriteDiagnostics(params *protocol.PublishDiagnosticsParams) {
	for _, diag := range params.Diagnostics {
		dw.ch <- &diag
	}
}

var _ = text.File((*BytesFile)(nil))

type BytesFile []byte

func (f *BytesFile) Reader() (io.Reader, error) {
	return bytes.NewReader(*f), nil
}

func (f *BytesFile) WriteAt(q0, q1 int, b []byte) (int, error) {
	r := []rune(string(*f))

	rr := make([]rune, 0, len(r)+len(b))
	rr = append(rr, r[:q0]...)
	rr = append(rr, []rune(string(b))...)
	rr = append(rr, r[q1:]...)
	*f = []byte(string(rr))
	return len(b), nil
}

func (f *BytesFile) Mark() error {
	return nil
}

func (f *BytesFile) DisableMark() error {
	return nil
}

func TestClientProvidesCodeAction(t *testing.T) {
	for _, tc := range []struct {
		provider interface{}
		kind     protocol.CodeActionKind
		want     bool
	}{
		{true, protocol.SourceOrganizeImports, true},
		{false, protocol.SourceOrganizeImports, false},
		{false, protocol.SourceOrganizeImports, false},
		{
			protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{protocol.QuickFix, protocol.SourceOrganizeImports}},
			protocol.SourceOrganizeImports,
			true,
		},
		{
			protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{protocol.QuickFix}},
			protocol.SourceOrganizeImports,
			false,
		},
		{
			protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{}},
			protocol.SourceOrganizeImports,
			false,
		},
		{
			protocol.CodeActionOptions{CodeActionKinds: nil},
			protocol.SourceOrganizeImports,
			false,
		},
	} {
		c := &Client{
			initializeResult: &protocol.InitializeResult{
				Capabilities: protocol.ServerCapabilities{
					CodeActionProvider: tc.provider,
				},
			},
		}
		got := lsp.ServerProvidesCodeAction(&c.initializeResult.Capabilities, tc.kind)
		want := tc.want
		if got != want {
			t.Errorf("got %v for provider %v and kind %v; want %v",
				got, tc.provider, tc.kind, want)
		}
	}
}
