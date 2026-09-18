package main

import (
	"io"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/creack/pty"
)

func TestParseArgs(t *testing.T) {
	t.Setenv("SHELL", "/opt/homebrew/bin/fish")
	got, err := parseArgs([]string{"--color", "-c", "printf", "blue"})
	if err != nil {
		t.Fatal(err)
	}
	want := options{
		colorMode: 1,
		command:   "printf blue",
		shell:     "/opt/homebrew/bin/fish",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseArgs() = %#v, want %#v", got, want)
	}
}

func TestParseShell(t *testing.T) {
	got, err := parseArgs([]string{"--shell", "/bin/sh", "--no-color"})
	if err != nil {
		t.Fatal(err)
	}
	if got.shell != "/bin/sh" || got.colorMode != 0 {
		t.Fatalf("unexpected options: %#v", got)
	}
}

func TestUnknownOption(t *testing.T) {
	if _, err := parseArgs([]string{"--wat"}); err == nil {
		t.Fatal("unknown option did not return an error")
	}
}

func TestExitCode(t *testing.T) {
	if code := exitCode(nil); code != 0 {
		t.Fatalf("exitCode(nil) = %d", code)
	}
	err := exec.Command("/bin/sh", "-c", "exit 7").Run()
	if code := exitCode(err); code != 7 {
		t.Fatalf("exitCode(exit 7) = %d", code)
	}
}

func TestInteractivePTY(t *testing.T) {
	child := exec.Command("/bin/sh")
	terminal, err := pty.Start(child)
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()

	if _, err := terminal.Write([]byte("printf 'blue 42\\n'\nexit\n")); err != nil {
		t.Fatal(err)
	}
	output, _ := io.ReadAll(terminal)
	if err := child.Wait(); err != nil {
		t.Fatalf("wrapped shell failed: %v\n%s", err, output)
	}
	text := string(output)
	if !strings.Contains(text, "blue 42") {
		t.Fatalf("PTY output does not contain command output: %q", text)
	}
}
