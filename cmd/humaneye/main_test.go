package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/youngwoocho02/human-eye-filter/internal/humaneye"
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

func TestShouldReduceUsesLineThreshold(t *testing.T) {
	options := humaneyeOptionsForTest()
	options.AutoMinLines = 2
	options.AutoMinChars = 1000

	if !shouldReduce("one\ntwo\n", options) {
		t.Fatal("expected two non-empty lines to trigger reduction")
	}
}

func TestShouldReduceKeepsShortOutputRaw(t *testing.T) {
	options := humaneyeOptionsForTest()
	options.AutoMinLines = 10
	options.AutoMinChars = 1000

	if shouldReduce("short output\n", options) {
		t.Fatal("expected short output to stay raw")
	}
}

func TestHookPowershellPrintsHelpers(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code, err := run([]string{"hook", "powershell"}, strings.NewReader(""), stdout, stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"function eye", "run-auto", "function eye-rg", "function eye-cm"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in hook output:\n%s", want, output)
		}
	}
}

func humaneyeOptionsForTest() humaneye.Options {
	return humaneye.DefaultOptions()
}
