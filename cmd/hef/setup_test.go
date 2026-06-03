package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPipeHookScriptWarnsWithoutHef(t *testing.T) {
	script := pipeHookScript()
	for _, want := range []string{"permissionDecision = \"deny\"", "permissionDecisionReason", "Add-CommandTexts", "parameters", "[no-hef]", "--no-hef", "#\\s*no-hef", "hef(?:\\.exe)?"} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected %q in setup hook script:\n%s", want, script)
		}
	}
	if strings.Contains(script, "updatedInput") {
		t.Fatalf("hook script should warn instead of rewriting command:\n%s", script)
	}
	if strings.Contains(script, "Error.WriteLine") || strings.Contains(script, "exit 2") {
		t.Fatalf("Codex hook script should deny with JSON stdout instead of failing:\n%s", script)
	}
	if strings.Contains(script, "humaneye") {
		t.Fatalf("hook script should use hef command name only:\n%s", script)
	}
}

func TestOpencodePluginWarnsWithoutHef(t *testing.T) {
	source := opencodePluginSource()
	for _, want := range []string{"export const hef", "throw new Error", "[no-hef]", "--no-hef", "#\\s*no-hef", "tool.execute.before"} {
		if !strings.Contains(source, want) {
			t.Fatalf("expected %q in OpenCode plugin:\n%s", want, source)
		}
	}
	if strings.Contains(source, `command + " | hef"`) {
		t.Fatalf("OpenCode plugin should warn instead of rewriting command:\n%s", source)
	}
	if strings.Contains(source, "humaneye") {
		t.Fatalf("OpenCode plugin should use hef command name only:\n%s", source)
	}
}

func TestSelectAgents(t *testing.T) {
	agents, err := selectAgents("all")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(agents, ",") != "claude,codex,opencode" {
		t.Fatalf("unexpected agents: %v", agents)
	}
}

func TestRemoveCodexHookBlockKeepsSiblingHooks(t *testing.T) {
	text := `[[hooks.PreToolUse]]

[[hooks.PreToolUse.hooks]]
type = "command"
command = 'powershell -File "C:\Users\young\.codex\hooks\block-cm-diff.ps1"'
timeout = 5

[[hooks.PreToolUse.hooks]]
type = "command"
command = 'powershell -File "C:\Users\young\.codex\hooks\warn-hef.ps1"'
timeout = 5

[hooks.state]
`

	result := removeCodexHookBlock(text, `powershell -File "C:\Users\young\.codex\hooks\warn-hef.ps1"`)
	if strings.Contains(result, "warn-hef.ps1") {
		t.Fatalf("expected warn-hef hook to be removed:\n%s", result)
	}
	if !strings.Contains(result, "block-cm-diff.ps1") {
		t.Fatalf("expected sibling hook to remain:\n%s", result)
	}
	if !strings.Contains(result, "[hooks.state]") {
		t.Fatalf("expected following table to remain:\n%s", result)
	}
}

func TestRemoveEmptyCodexPreToolUseBlocks(t *testing.T) {
	text := `[[hooks.PreToolUse]]

[[hooks.PreToolUse.hooks]]
type = "command"
command = 'powershell -File "C:\Users\young\.codex\hooks\block-cm-diff.ps1"'
timeout = 5

[[hooks.PreToolUse]]
matcher = "^Bash$"

[[hooks.PreToolUse]]
matcher = "^(Bash|exec|shell_command)$"

[[hooks.PreToolUse.hooks]]
type = "command"
command = 'powershell -File "C:\Users\young\.codex\hooks\hef-pipe.ps1"'
timeout = 5
`

	result := removeEmptyCodexPreToolUseBlocks(text)
	if strings.Contains(result, `matcher = "^Bash$"`) {
		t.Fatalf("expected empty matcher-only block to be removed:\n%s", result)
	}
	if !strings.Contains(result, "block-cm-diff.ps1") {
		t.Fatalf("expected sibling hook block to remain:\n%s", result)
	}
	if !strings.Contains(result, "hef-pipe.ps1") {
		t.Fatalf("expected hef hook block to remain:\n%s", result)
	}
}

func TestCodexHookUsesBashMatcher(t *testing.T) {
	dir := t.TempDir()
	summary, err := setupCodex(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if summary != "installed PreToolUse hook" {
		t.Fatalf("unexpected summary: %s", summary)
	}

	configPath := filepath.Join(dir, ".codex", "config.toml")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `matcher = "^Bash$"`) {
		t.Fatalf("expected Codex hef hook to match Bash only:\n%s", text)
	}
	if strings.Contains(text, "exec|shell_command") {
		t.Fatalf("expected Codex hef hook not to use non-canonical tool aliases:\n%s", text)
	}
	if !strings.Contains(text, "hef-pipe.ps1") {
		t.Fatalf("expected Codex hef hook command:\n%s", text)
	}
}
