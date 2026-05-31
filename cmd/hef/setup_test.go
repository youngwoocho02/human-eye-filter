package main

import (
	"strings"
	"testing"
)

func TestPipeHookScriptAppendsHef(t *testing.T) {
	script := pipeHookScript()
	for _, want := range []string{"| hef", "hef(?:\\.exe)?", "PreToolUse"} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected %q in setup hook script:\n%s", want, script)
		}
	}
	if strings.Contains(script, "humaneye") {
		t.Fatalf("hook script should use hef command name only:\n%s", script)
	}
}

func TestOpencodePluginAppendsHef(t *testing.T) {
	source := opencodePluginSource()
	for _, want := range []string{"export const hef", "| hef", "tool.execute.before"} {
		if !strings.Contains(source, want) {
			t.Fatalf("expected %q in OpenCode plugin:\n%s", want, source)
		}
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
