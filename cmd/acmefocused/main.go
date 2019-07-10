// Program acmefocused is a server that tells acme's focused window ID
// to clients.
//
// Acmefocus will listen on a unix domain socket at NAMESPACE/acmefocused.
// The window ID is written to a client and the connection to the client
// is closed immediately. The window ID is useful for acme clients that
// expects $winid environment variable to be defined but it is being
// run outside of acme.
//
// Usage:
//	$ acme &
//	$ acmefocused &
//	$ dial $(namespace)/acmefocused
//	1
//	$
//
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"9fans.net/go/acme"
	"9fans.net/go/plan9/client"
)

func main() {
	var fw focusedWin
	go fw.readLog()

	listenAndServe(listenAddr(), func(conn net.Conn) {
		fmt.Fprintf(conn, "%d\n", fw.ID())
		conn.Close()
	})
}

type focusedWin struct {
	id int
	mu sync.Mutex
}

// ID returns the window ID of currently focused window.
func (fw *focusedWin) ID() int {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.id
}

func (fw *focusedWin) readLog() {
	alog, err := acme.Log()
	if err != nil {
		log.Fatalf("failed to open acme log: %v\n", err)
	}
	defer alog.Close()
	for {
		ev, err := alog.Read()
		if err != nil {
			log.Fatalf("acme log read failed: %v\n", err)
		}
		if ev.Op == "focus" {
			fw.mu.Lock()
			fw.id = ev.ID
			fw.mu.Unlock()
		}
	}
}

func listenAddr() string {
	return filepath.Join(client.Namespace(), "acmefocused")
}

func listenAndServe(addr string, handle func(net.Conn)) {
	ln, err := Listen("unix", addr)
	if err != nil {
		log.Fatalf("listen failed: %v\n", err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("accept failed: %v\n", err)
		}
		go handle(conn)
	}
}

// Listen is the same as net.Listen but it reuses dead unix domain socket.
func Listen(network, address string) (net.Listener, error) {
	ln, err := net.Listen(network, address)
	if err != nil && network == "unix" && isAddrInUse(err) {
		if _, err1 := net.Dial(network, address); !isConnRefused(err1) {
			return nil, err // Listen error
		}
		// Dead socket, so remove it.
		err = os.Remove(address)
		if err != nil {
			return nil, err
		}
		ln, err = net.Listen(network, address)
	}
	return ln, err
}

func isAddrInUse(err error) bool {
	if err, ok := err.(*net.OpError); ok {
		if err, ok := err.Err.(*os.SyscallError); ok {
			return err.Err == syscall.EADDRINUSE
		}
	}
	return false
}

func isConnRefused(err error) bool {
	if err, ok := err.(*net.OpError); ok {
		if err, ok := err.Err.(*os.SyscallError); ok {
			return err.Err == syscall.ECONNREFUSED
		}
	}
	return false
}
