package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runSetup(args []string, stdout, stderr io.Writer) (int, error) {
	flags := flag.NewFlagSet("hef setup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	agent := flags.String("agent", "all", "agent to configure: claude, codex, opencode, all")
	remove := flags.Bool("remove", false, "remove hef hook/plugin setup")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, nil
		}
		return 2, err
	}

	agents, err := selectAgents(*agent)
	if err != nil {
		return 2, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return 1, err
	}

	for _, current := range agents {
		var summary string
		var setupErr error
		switch current {
		case "claude":
			summary, setupErr = setupClaude(home, *remove)
		case "codex":
			summary, setupErr = setupCodex(home, *remove)
		case "opencode":
			summary, setupErr = setupOpencode(home, *remove)
		}
		if setupErr != nil {
			return 1, fmt.Errorf("%s: %w", current, setupErr)
		}
		if _, err := fmt.Fprintf(stdout, "%s: %s\n", current, summary); err != nil {
			return 1, err
		}
	}

	return 0, nil
}

func selectAgents(agent string) ([]string, error) {
	switch agent {
	case "all":
		return []string{"claude", "codex", "opencode"}, nil
	case "claude", "codex", "opencode":
		return []string{agent}, nil
	default:
		return nil, fmt.Errorf("unknown agent %q (want claude, codex, opencode, all)", agent)
	}
}

func setupClaude(home string, remove bool) (string, error) {
	hookPath := filepath.Join(home, ".claude", "hooks", "hef-pipe.ps1")
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	command := powerShellHookCommand(hookPath)

	if remove {
		return removeClaudeHook(settingsPath, hookPath, command)
	}

	if err := writeText(hookPath, pipeHookScript()); err != nil {
		return "", err
	}

	settings, err := readJSONObject(settingsPath)
	if err != nil {
		return "", err
	}

	hooks := objectValue(settings, "hooks")
	preToolUse := arrayValue(hooks, "PreToolUse")
	if !claudeHookPresent(preToolUse, command) {
		entry := map[string]any{
			"matcher": "Bash",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": command,
				},
			},
		}
		preToolUse = append(preToolUse, entry)
	}
	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	if err := writeJSON(settingsPath, settings); err != nil {
		return "", err
	}
	return "installed PreToolUse hook", nil
}

func removeClaudeHook(settingsPath, hookPath, command string) (string, error) {
	changed := false
	settings, err := readJSONObject(settingsPath)
	if err != nil {
		return "", err
	}
	if len(settings) > 0 {
		if hooks, ok := settings["hooks"].(map[string]any); ok {
			if preToolUse, ok := hooks["PreToolUse"].([]any); ok {
				kept := removeClaudeHookCommand(preToolUse, command)
				if len(kept) != len(preToolUse) {
					changed = true
					if len(kept) == 0 {
						delete(hooks, "PreToolUse")
					} else {
						hooks["PreToolUse"] = kept
					}
					if len(hooks) == 0 {
						delete(settings, "hooks")
					} else {
						settings["hooks"] = hooks
					}
					if err := writeJSON(settingsPath, settings); err != nil {
						return "", err
					}
				}
			}
		}
	}
	if err := removeFileIfExists(hookPath); err != nil {
		return "", err
	}
	if changed {
		return "removed PreToolUse hook", nil
	}
	return "no change", nil
}

func setupCodex(home string, remove bool) (string, error) {
	hookPath := filepath.Join(home, ".codex", "hooks", "hef-pipe.ps1")
	configPath := filepath.Join(home, ".codex", "config.toml")
	command := powerShellHookCommand(hookPath)

	if remove {
		if err := removeFileIfExists(hookPath); err != nil {
			return "", err
		}
		return removeCodexHook(configPath, command)
	}

	if err := writeText(hookPath, pipeHookScript()); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	text := string(raw)
	if strings.Contains(text, "command = '"+command+"'") {
		return "hook already configured", nil
	}
	block := "\n[[hooks.PreToolUse]]\n\n[[hooks.PreToolUse.hooks]]\ntype = \"command\"\ncommand = '" + command + "'\ntimeout = 5\n"
	if strings.TrimSpace(text) == "" {
		text = strings.TrimLeft(block, "\n")
	} else {
		text = strings.TrimRight(text, "\r\n") + "\n" + block
	}
	if err := writeText(configPath, text); err != nil {
		return "", err
	}
	return "installed PreToolUse hook", nil
}

func removeCodexHook(configPath, command string) (string, error) {
	raw, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return "no change", nil
	}
	if err != nil {
		return "", err
	}
	text := string(raw)
	if !strings.Contains(text, command) {
		return "no change", nil
	}
	lines := strings.Split(text, "\n")
	cmdLine := -1
	for i, line := range lines {
		if strings.Contains(line, command) {
			cmdLine = i
			break
		}
	}
	if cmdLine < 0 {
		return "no change", nil
	}
	start := cmdLine
	for start >= 0 && strings.TrimSpace(lines[start]) != "[[hooks.PreToolUse]]" {
		start--
	}
	if start < 0 {
		return "left config unchanged; manual removal needed", nil
	}
	end := cmdLine + 1
	for end < len(lines) {
		trimmed := strings.TrimSpace(lines[end])
		if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[[hooks.PreToolUse.hooks]]") {
			break
		}
		end++
	}
	for start > 0 && strings.TrimSpace(lines[start-1]) == "" {
		start--
	}
	kept := append([]string{}, lines[:start]...)
	kept = append(kept, lines[end:]...)
	result := strings.TrimRight(strings.Join(kept, "\n"), "\r\n")
	if result != "" {
		result += "\n"
	}
	if err := writeText(configPath, result); err != nil {
		return "", err
	}
	return "removed PreToolUse hook", nil
}

func setupOpencode(home string, remove bool) (string, error) {
	dir := filepath.Join(home, ".config", "opencode")
	pluginPath := filepath.Join(dir, "plugins", "hef.ts")
	configPath := filepath.Join(dir, "opencode.json")
	entry := "./plugins/hef.ts"

	if remove {
		if err := removeFileIfExists(pluginPath); err != nil {
			return "", err
		}
		config, err := readJSONObject(configPath)
		if err != nil {
			return "", err
		}
		plugins := removeString(arrayValue(config, "plugin"), entry)
		if len(plugins) == 0 {
			delete(config, "plugin")
		} else {
			config["plugin"] = plugins
		}
		if len(config) > 0 {
			if err := writeJSON(configPath, config); err != nil {
				return "", err
			}
		}
		return "removed plugin", nil
	}

	if err := writeText(pluginPath, opencodePluginSource()); err != nil {
		return "", err
	}
	config, err := readJSONObject(configPath)
	if err != nil {
		return "", err
	}
	plugins := arrayValue(config, "plugin")
	if !containsString(plugins, entry) {
		plugins = append(plugins, entry)
	}
	config["plugin"] = plugins
	if err := writeJSON(configPath, config); err != nil {
		return "", err
	}
	return "installed plugin", nil
}

func pipeHookScript() string {
	return `$ErrorActionPreference = "Stop"

$stdinText = [Console]::In.ReadToEnd()
if ([string]::IsNullOrWhiteSpace($stdinText)) {
    exit 0
}

try {
    $hookInput = $stdinText | ConvertFrom-Json
} catch {
    exit 0
}

$toolInput = $hookInput.PSObject.Properties["tool_input"].Value
if ($null -eq $toolInput) {
    exit 0
}

$commandProperty = $toolInput.PSObject.Properties["command"]
if ($null -eq $commandProperty -or -not ($commandProperty.Value -is [string])) {
    exit 0
}

$command = $commandProperty.Value
if ([string]::IsNullOrWhiteSpace($command)) {
    exit 0
}

$backgroundProperty = $toolInput.PSObject.Properties["run_in_background"]
if ($null -ne $backgroundProperty -and $backgroundProperty.Value -eq $true) {
    exit 0
}

if ($command -match "(?i)(^|\|\s*&?\s*)hef(?:\.exe)?\b") {
    exit 0
}

$output = @{
    hookSpecificOutput = @{
        hookEventName = "PreToolUse"
        permissionDecision = "allow"
        updatedInput = @{
            command = "$command | hef"
        }
    }
}

[Console]::Out.WriteLine(($output | ConvertTo-Json -Compress))
exit 0
`
}

func opencodePluginSource() string {
	return `import type { Plugin } from "@opencode-ai/plugin"

export const hef: Plugin = async () => {
  return {
    "tool.execute.before": async (input, output) => {
      const tool = input.tool
      if (tool !== "bash" && tool !== "shell") return

      const command = output.args?.command
      if (typeof command !== "string" || command.trim().length === 0) return
      if (/\|\s*&?\s*hef(?:\.exe)?\b/i.test(command)) return

      output.args.command = command + " | hef"
    },
  }
}

export default hef
`
}

func powerShellHookCommand(path string) string {
	return `powershell -NoProfile -ExecutionPolicy Bypass -File "` + path + `"`
}

func readJSONObject(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(raw)) == "" {
		return map[string]any{}, nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return value, nil
}

func objectValue(parent map[string]any, key string) map[string]any {
	value, ok := parent[key].(map[string]any)
	if !ok {
		value = map[string]any{}
		parent[key] = value
	}
	return value
}

func arrayValue(parent map[string]any, key string) []any {
	value, ok := parent[key].([]any)
	if !ok {
		return []any{}
	}
	return value
}

func claudeHookPresent(preToolUse []any, command string) bool {
	for _, item := range preToolUse {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := entry["hooks"].([]any)
		if !ok {
			continue
		}
		for _, hook := range hooks {
			hookEntry, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			if hookEntry["command"] == command {
				return true
			}
		}
	}
	return false
}

func removeClaudeHookCommand(preToolUse []any, command string) []any {
	keptEntries := make([]any, 0, len(preToolUse))
	for _, item := range preToolUse {
		entry, ok := item.(map[string]any)
		if !ok {
			keptEntries = append(keptEntries, item)
			continue
		}
		hooks, ok := entry["hooks"].([]any)
		if !ok {
			keptEntries = append(keptEntries, item)
			continue
		}
		keptHooks := make([]any, 0, len(hooks))
		for _, hook := range hooks {
			hookEntry, ok := hook.(map[string]any)
			if !ok || hookEntry["command"] != command {
				keptHooks = append(keptHooks, hook)
			}
		}
		if len(keptHooks) == 0 {
			continue
		}
		entry["hooks"] = keptHooks
		keptEntries = append(keptEntries, entry)
	}
	return keptEntries
}

func containsString(values []any, target string) bool {
	for _, value := range values {
		if text, ok := value.(string); ok && text == target {
			return true
		}
	}
	return false
}

func removeString(values []any, target string) []any {
	kept := make([]any, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok && text == target {
			continue
		}
		kept = append(kept, value)
	}
	return kept
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeText(path, string(data)+"\n")
}

func writeText(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
