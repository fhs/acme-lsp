package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const testWinID = "42"

func TestMain(m *testing.M) {
	switch os.Getenv("TEST_MAIN") {
	case "acmefocused":
		listenAndServe(listenAddr(), func(conn net.Conn) {
			fmt.Fprintf(conn, "%s\n", testWinID)
			conn.Close()
		})
	default:
		os.Exit(m.Run())
	}
}

func TestListenAndServe(t *testing.T) {
	dir, err := ioutil.TempDir("", "acmefocused-test")
	if err != nil {
		t.Fatalf("couldn't create temporary directory: %v", err)
	}
	defer os.RemoveAll(dir)
	os.Setenv("NAMESPACE", dir)
	addr := listenAddr()

	cmd := exec.Command(os.Args[0])
	cmd.Env = append(
		os.Environ(),
		"TEST_MAIN=acmefocused",
		fmt.Sprintf("NAMESPACE=%v", dir),
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	killed := make(chan struct{})
	go func() {
		err := cmd.Run()
		if e, ok := err.(*exec.ExitError); !ok || !strings.Contains(e.Error(), "killed") {
			t.Errorf("process exited with error %v; want exit due to kill", err)
		}
		close(killed)
	}()

	for i := 0; i < 10; i++ {
		conn, err := net.Dial("unix", addr)
		if err != nil {
			if i >= 9 {
				t.Fatalf("dial failed after multiple attempts: %v", err)
			}
			time.Sleep(time.Second)
			continue
		}
		want := []byte(testWinID + "\n")
		got, err := ioutil.ReadAll(conn)
		if err != nil {
			t.Errorf("read failed: %v", err)
		}
		if !bytes.Equal(want, got) {
			t.Errorf("got bytes %q; want %q", got, want)
		}
		break
	}

	err = cmd.Process.Kill()
	if err != nil {
		t.Fatalf("kill failed: %v", err)
	}
	<-killed

	// This should reuse the unix domain socket.
	_, err = Listen("unix", addr)
	if err != nil {
		t.Errorf("second listen returned error %v", err)
	}
}
