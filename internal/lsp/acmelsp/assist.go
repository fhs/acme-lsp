package acmelsp

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"
	"unicode"

	"github.com/fhs/acme-lsp/internal/acme"
	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
	"github.com/fhs/acme-lsp/internal/lsp/text"
)

func watchLog(ch chan<- *acme.LogEvent) {
	alog, err := acme.Log()
	if err != nil {
		panic(err)
	}
	defer alog.Close()
	for {
		ev, err := alog.Read()
		if err != nil {
			panic(err)
		}
		ch <- &ev
	}
}

// focusWindow represents the last focused window.
//
// Note that we can't cache the *acmeutil.Win for the window
// because having the ctl file open prevents del event from
// being delivered to acme/log file.
type focusWin struct {
	id   int
	q0   int
	name string
}

func newFocusWin() *focusWin {
	var fw focusWin
	fw.Reset()
	return &fw
}

func (fw *focusWin) Reset() {
	fw.id = -1
	fw.q0 = -1
	fw.name = ""
}

func (fw *focusWin) SetQ0() bool {
	if fw.id < 0 {
		return false
	}
	w, err := acmeutil.OpenWin(fw.id)
	if err != nil {
		return false
	}
	defer w.CloseFiles()

	q0, _, err := w.CurrentAddr()
	if err != nil {
		return false
	}
	fw.q0 = q0
	return true
}

func notifyPosChange(sm ServerMatcher, ch chan<- *focusWin) {
	fw := newFocusWin()
	logch := make(chan *acme.LogEvent)
	go watchLog(logch)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	pos := make(map[int]int) // winid -> q0

	for {
		select {
		case ev := <-logch:
			_, found, err := sm.ServerMatch(context.Background(), ev.Name)
			if found && err == nil && ev.Op == "focus" {
				// TODO(fhs): we should really make use of context
				// and cancel outstanding rpc requests on previously focused window.
				fw.id = ev.ID
				fw.name = ev.Name
			} else {
				fw.Reset()
			}

		case <-ticker.C:
			if fw.SetQ0() && pos[fw.id] != fw.q0 {
				pos[fw.id] = fw.q0
				ch <- &focusWin{ // send a copy
					id:   fw.id,
					q0:   fw.q0,
					name: fw.name,
				}
			}
		}
	}
}

type outputWin struct {
	*acmeutil.Win
	body  io.Writer
	event <-chan *acme.Event
	sm    ServerMatcher
}

func newOutputWin(sm ServerMatcher, name string) (*outputWin, error) {
	w, err := acmeutil.NewWin()
	if err != nil {
		return nil, err
	}
	w.Name(name)
	return &outputWin{
		Win:   w,
		body:  w.FileReadWriter("body"),
		event: w.EventChan(),
		sm:    sm,
	}, nil
}

func (w *outputWin) Close() {
	w.Del(true)
	w.CloseFiles()
}

func readLeftRight(id int, q0 int) (left, right rune, err error) {
	w, err := acmeutil.OpenWin(id)
	if err != nil {
		return 0, 0, err
	}
	defer w.CloseFiles()

	err = w.Addr("#%v,#%v", q0-1, q0+1)
	if err != nil {
		return 0, 0, err
	}

	b, err := ioutil.ReadAll(w.FileReadWriter("xdata"))
	if err != nil {
		return 0, 0, err
	}
	r := []rune(string(b))
	if len(r) != 2 {
		// TODO(fhs): deal with EOF and beginning of file
		return 0, 0, fmt.Errorf("could not find rune left and right of cursor")
	}
	return r[0], r[1], nil
}

func winPosition(id int) (*protocol.TextDocumentPositionParams, string, error) {
	w, err := acmeutil.OpenWin(id)
	if err != nil {
		return nil, "", err
	}
	defer w.CloseFiles()

	return text.Position(w)
}

func isIdentifier(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_'
}

func helpType(left, right rune) string {
	if unicode.IsSpace(left) && unicode.IsSpace(right) {
		return ""
	}
	if isIdentifier(left) && isIdentifier(right) {
		return "hov"
	}
	switch left {
	case '(', '[', '<', '{':
		return "sig"
	}
	return "comp"
}

func dprintf(format string, args ...interface{}) {
	if Verbose {
		log.Printf(format, args...)
	}
}

// Update writes result of cmd to output window.
func (w *outputWin) Update(fw *focusWin, server proxy.Server, cmd string) {
	if cmd == "auto" {
		left, right, err := readLeftRight(fw.id, fw.q0)
		if err != nil {
			dprintf("read left/right rune: %v\n", err)
			return
		}
		cmd = helpType(left, right)
		if cmd == "" {
			return
		}
	}

	rc := NewRemoteCmd(server, fw.id, -1)
	rc.Stdout = w.body
	rc.Stderr = w.body
	ctx := context.Background()

	// Assume file is already opened by file management.
	err := rc.DidChange(ctx)
	if err != nil {
		dprintf("DidChange failed: %v\n", err)
		return
	}

	w.Clear()
	switch cmd {
	case "comp":
		err := rc.Completion(ctx, false)
		if err != nil {
			dprintf("Completion failed: %v\n", err)
		}

	case "sig":
		err = rc.SignatureHelp(ctx)
		if err != nil {
			dprintf("SignatureHelp failed: %v\n", err)
		}
	case "hov":
		err = rc.Hover(ctx)
		if err != nil {
			dprintf("Hover failed: %v\n", err)
		}
	default:
		log.Fatalf("invalid command %q\n", cmd)
	}
	w.Ctl("clean")
}

// Assist creates an acme window where output of cmd is written after each
// cursor position change in acme. Cmd is either "comp", "sig", "hov", or "auto"
// for completion, signature help, hover, or auto-detection of the former three.
func Assist(sm ServerMatcher, cmd string) error {
	name := "/LSP/assist"
	if cmd != "auto" {
		name += "/" + cmd
	}
	w, err := newOutputWin(sm, name)
	if err != nil {
		return fmt.Errorf("failed to create acme window: %v", err)
	}
	defer w.Close()

	fch := make(chan *focusWin)
	go notifyPosChange(sm, fch)

loop:
	for {
		select {
		case fw := <-fch:
			server, found, err := sm.ServerMatch(context.Background(), fw.name)
			if err != nil {
				log.Printf("failed to start language server: %v\n", err)
			}
			if found {
				w.Update(fw, server, cmd)
			}

		case ev := <-w.event:
			if ev == nil {
				break loop
			}
			switch ev.C2 {
			case 'x', 'X': // execute
				if string(ev.Text) == "Del" {
					break loop
				}
			}
			w.WriteEvent(ev)
		}
	}
	return nil
}

// ServerMatcher represents a set of servers where it's possible to
// find a matching server based on filename.
type ServerMatcher interface {
	ServerMatch(ctx context.Context, filename string) (proxy.Server, bool, error)
}

// UnitServerMatcher implements ServerMatcher using only one server.
type UnitServerMatcher struct {
	proxy.Server
}

func (sm *UnitServerMatcher) ServerMatch(ctx context.Context, filename string) (proxy.Server, bool, error) {
	_, err := sm.Server.InitializeResult(ctx, &protocol.TextDocumentIdentifier{
		URI: text.ToURI(filename),
	})
	if err != nil {
		return nil, false, err
	}
	return sm.Server, true, nil
}
