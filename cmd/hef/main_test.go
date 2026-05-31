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

func TestReadBoundedReportsOmittedBytes(t *testing.T) {
	input, err := readBounded(strings.NewReader("1234567890"), 4)
	if err != nil {
		t.Fatal(err)
	}

	output := string(input)
	for _, want := range []string{"input limit reached", "omitted 6 more raw bytes", "Narrow the command"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}
