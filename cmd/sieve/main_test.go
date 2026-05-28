package main

import (
	"strings"
	"testing"
)

func TestReadBoundedTrimsUTF8(t *testing.T) {
	input, err := readBounded(strings.NewReader("가나다"), 4)
	if err != nil {
		t.Fatal(err)
	}

	output := string(input)
	if strings.Contains(output, "\ufffd") {
		t.Fatalf("invalid utf8 replacement appeared: %q", output)
	}
	if !strings.Contains(output, "가") {
		t.Fatalf("expected first full rune to remain: %q", output)
	}
}

func TestBoundedOutputStringTrimsUTF8(t *testing.T) {
	output := newBoundedOutput(4)
	_, err := output.Write([]byte("가나다"))
	if err != nil {
		t.Fatal(err)
	}

	text := output.String()
	if strings.Contains(text, "\ufffd") {
		t.Fatalf("invalid utf8 replacement appeared: %q", text)
	}
	if !strings.Contains(text, "가") {
		t.Fatalf("expected first full rune to remain: %q", text)
	}
}
