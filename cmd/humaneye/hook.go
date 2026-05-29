package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
)

const hookUsage = "usage: humaneye hook <powershell|claude|codex|install|uninstall>"

// runHook dispatches the `humaneye hook ...` subcommands.
func runHook(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, errors.New(hookUsage)
	}

	switch args[0] {
	case "powershell":
		if len(args) != 1 {
			return 2, errors.New("usage: humaneye hook powershell")
		}
		if _, err := io.WriteString(stdout, powershellHookScript()); err != nil {
			return 1, err
		}
		return 0, nil
	case "claude", "codex":
		if len(args) != 1 {
			return 2, errors.New("usage: humaneye hook " + args[0])
		}
		return runPreToolUseHook(stdin, stdout)
	case "install", "uninstall":
		return runInstall(args[0], args[1:], stdout, stderr)
	default:
		return 2, errors.New(hookUsage)
	}
}

// preToolUseInput is the PreToolUse hook payload shared by Claude Code and Codex.
type preToolUseInput struct {
	ToolInput struct {
		Command         string `json:"command"`
		RunInBackground bool   `json:"run_in_background"`
	} `json:"tool_input"`
}

type preToolUseOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName      string       `json:"hookEventName"`
	PermissionDecision string       `json:"permissionDecision"`
	UpdatedInput       updatedInput `json:"updatedInput"`
}

type updatedInput struct {
	Command string `json:"command"`
}

// runPreToolUseHook reads a PreToolUse payload from stdin and, for foreground Bash
// commands, wraps them in `humaneye run-auto`. It fails open: any problem yields
// (0, nil) with no stdout so the Bash tool is never blocked.
func runPreToolUseHook(stdin io.Reader, stdout io.Writer) (int, error) {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return 0, nil
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	var input preToolUseInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return 0, nil
	}

	command := input.ToolInput.Command
	if command == "" {
		return 0, nil
	}

	self, err := os.Executable()
	if err != nil {
		return 0, nil
	}
	selfFwd := strings.ReplaceAll(self, "\\", "/")

	trimmed := strings.TrimLeft(command, " \t")
	if input.ToolInput.RunInBackground ||
		strings.HasPrefix(trimmed, selfFwd) ||
		strings.HasPrefix(trimmed, "'"+selfFwd) {
		return 0, nil
	}

	wrapped := "'" + selfFwd + "' run-auto -- bash -c " + bashSingleQuote(command)

	out := preToolUseOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
			UpdatedInput:       updatedInput{Command: wrapped},
		},
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return 0, nil
	}
	if _, err := stdout.Write(encoded); err != nil {
		return 1, err
	}
	return 0, nil
}

// bashSingleQuote wraps s in single quotes safe for bash, escaping embedded quotes.
func bashSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func powershellHookScript() string {
	return `# HumanEye PowerShell hook helpers.
# Policy: short command output stays raw; long output is reduced through humaneye run-auto.

function Invoke-HumanEye {
    humaneye run-auto -- @args
}

function eye {
    humaneye run-auto -- @args
}

function eye-rg {
    humaneye --tool rg run-auto -- rg @args
}

function eye-git {
    humaneye --tool git run-auto -- git @args
}

function eye-cm {
    humaneye --tool cm run-auto -- cm @args
}

function eye-unity {
    humaneye --tool unity-cli run-auto -- unity-cli @args
}
`
}
