package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBashSingleQuote(t *testing.T) {
	cases := map[string]string{
		"echo hi":   "'echo hi'",
		"it's":      "'it'\\''s'",
		"a'b'c":     "'a'\\''b'\\''c'",
		"":          "''",
		"no quotes": "'no quotes'",
		"a\nb":      "'a\nb'",
	}
	for in, want := range cases {
		if got := bashSingleQuote(in); got != want {
			t.Errorf("bashSingleQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func runHookClaude(t *testing.T, payload string) string {
	t.Helper()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	code, err := run([]string{"hook", "claude"}, strings.NewReader(payload), &out, &errBuf)
	if err != nil {
		t.Fatalf("run returned err: %v", err)
	}
	if code != 0 {
		t.Fatalf("run returned code %d, want 0", code)
	}
	return out.String()
}

func TestHookClaudeWrapsCommand(t *testing.T) {
	out := runHookClaude(t, `{"tool_input":{"command":"ls -la"}}`)
	if out == "" {
		t.Fatal("expected stdout, got empty")
	}

	var parsed preToolUseOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, out)
	}
	hs := parsed.HookSpecificOutput
	if hs.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName = %q", hs.HookEventName)
	}
	if hs.PermissionDecision != "allow" {
		t.Errorf("permissionDecision = %q", hs.PermissionDecision)
	}

	self, _ := os.Executable()
	selfFwd := strings.ReplaceAll(self, "\\", "/")
	want := "'" + selfFwd + "' run-auto -- bash -c " + bashSingleQuote("ls -la")
	if hs.UpdatedInput.Command != want {
		t.Errorf("wrapped command = %q, want %q", hs.UpdatedInput.Command, want)
	}

	// Compact JSON: no indentation newlines.
	if strings.Contains(out, "\n  ") {
		t.Errorf("expected compact JSON, got %q", out)
	}
}

func TestHookCodexSharesImplementation(t *testing.T) {
	var out bytes.Buffer
	code, err := run([]string{"hook", "codex"}, strings.NewReader(`{"tool_input":{"command":"echo hi"}}`), &out, &bytes.Buffer{})
	if err != nil || code != 0 {
		t.Fatalf("run code=%d err=%v", code, err)
	}
	if !strings.Contains(out.String(), "run-auto -- bash -c") {
		t.Errorf("codex did not wrap: %q", out.String())
	}
}

func TestHookClaudeSkipsBackground(t *testing.T) {
	out := runHookClaude(t, `{"tool_input":{"command":"sleep 100","run_in_background":true}}`)
	if out != "" {
		t.Errorf("expected no stdout for background command, got %q", out)
	}
}

func TestHookClaudeSkipsAlreadyWrapped(t *testing.T) {
	self, _ := os.Executable()
	selfFwd := strings.ReplaceAll(self, "\\", "/")

	bare := runHookClaude(t, `{"tool_input":{"command":`+jsonString(selfFwd+" run-auto -- bash -c 'ls'")+`}}`)
	if bare != "" {
		t.Errorf("expected no stdout for already-wrapped (bare) command, got %q", bare)
	}

	quoted := runHookClaude(t, `{"tool_input":{"command":`+jsonString("'"+selfFwd+"' run-auto -- bash -c 'ls'")+`}}`)
	if quoted != "" {
		t.Errorf("expected no stdout for already-wrapped (quoted) command, got %q", quoted)
	}
}

func TestHookClaudeEmptyAndBadInput(t *testing.T) {
	if got := runHookClaude(t, `{"tool_input":{"command":""}}`); got != "" {
		t.Errorf("empty command should produce no stdout, got %q", got)
	}
	if got := runHookClaude(t, `not json`); got != "" {
		t.Errorf("invalid JSON should fail open with no stdout, got %q", got)
	}
}

func TestHookClaudeStripsBOM(t *testing.T) {
	payload := "\xEF\xBB\xBF" + `{"tool_input":{"command":"pwd"}}`
	out := runHookClaude(t, payload)
	if !strings.Contains(out, "run-auto -- bash -c") {
		t.Errorf("BOM-prefixed payload not handled: %q", out)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
