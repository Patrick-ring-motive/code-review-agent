package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestColorize(t *testing.T) {
	result := colorize("blue 42", true)
	if !strings.Contains(result, "\x1b[38;2;78;78;255mblue\x1b[0m") {
		t.Fatalf("blue was not colored: %q", result)
	}
	if !strings.Contains(result, "\x1b[38;2;0;191;255m42\x1b[0m") {
		t.Fatalf("number was not colored: %q", result)
	}
	if result := colorize("blue 42", false); result != "blue 42" {
		t.Fatalf("disabled colorizer changed text: %q", result)
	}
}

func TestColorWriterPreservesTerminalSequences(t *testing.T) {
	var output bytes.Buffer
	writer := newColorWriter(&output, true)
	for _, part := range []string{"\x1b]7;file://host/", "path\x07", "fish prompt"} {
		if _, err := writer.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.flush(); err != nil {
		t.Fatal(err)
	}
	expected := "\x1b]7;file://host/path\x07fish prompt"
	if output.String() != expected {
		t.Fatalf("terminal sequence changed:\n got %q\nwant %q", output.String(), expected)
	}
}

func TestColorWriterColorsOutputAfterFishStyle(t *testing.T) {
	var output bytes.Buffer
	writer := newColorWriter(&output, true)
	if _, err := writer.Write([]byte("\x1b[32mprintf\x1b[0m\nblue 42\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.flush(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\x1b[38;2;78;78;255mblue\x1b[0m") {
		t.Fatalf("plain output after Fish styling was not colored: %q", output.String())
	}
}

func TestDynamicSymbolColor(t *testing.T) {
	result := colorize("+", true)
	if result == "+" || !strings.Contains(result, "+") {
		t.Fatalf("symbol was not colored: %q", result)
	}
}
