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
	legacyHookPaths := []string{
		filepath.Join(home, ".claude", "hooks", "warn-hef.ps1"),
	}

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
	for _, legacyPath := range legacyHookPaths {
		legacyCommand := powerShellHookCommand(legacyPath)
		preToolUse = removeClaudeHookCommand(preToolUse, legacyCommand)
		if err := removeFileIfExists(legacyPath); err != nil {
			return "", err
		}
	}
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
	legacyHookPaths := []string{
		filepath.Join(home, ".codex", "hooks", "warn-hef.ps1"),
	}

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
	for _, legacyPath := range legacyHookPaths {
		legacyCommand := powerShellHookCommand(legacyPath)
		text = removeCodexHookBlock(text, legacyCommand)
		if err := removeFileIfExists(legacyPath); err != nil {
			return "", err
		}
	}
	text = removeCodexHookBlock(text, command)
	text = removeEmptyCodexPreToolUseBlocks(text)
	block := "\n[[hooks.PreToolUse]]\nmatcher = \"^Bash$\"\n\n[[hooks.PreToolUse.hooks]]\ntype = \"command\"\ncommand = '" + command + "'\ncommand_windows = '" + command + "'\ntimeout = 5\n"
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
	result := removeCodexHookBlock(text, command)
	if result == text {
		return "left config unchanged; manual removal needed", nil
	}
	result = removeEmptyCodexPreToolUseBlocks(result)
	if err := writeText(configPath, result); err != nil {
		return "", err
	}
	return "removed PreToolUse hook", nil
}

func removeCodexHookBlock(text, command string) string {
	lines := strings.Split(text, "\n")
	cmdLine := -1
	for i, line := range lines {
		if strings.Contains(line, command) {
			cmdLine = i
			break
		}
	}
	if cmdLine < 0 {
		return text
	}
	start := cmdLine
	for start >= 0 && strings.TrimSpace(lines[start]) != "[[hooks.PreToolUse.hooks]]" {
		start--
	}
	if start < 0 {
		return text
	}
	end := cmdLine + 1
	for end < len(lines) {
		trimmed := strings.TrimSpace(lines[end])
		if strings.HasPrefix(trimmed, "[") {
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
	return result
}

func removeEmptyCodexPreToolUseBlocks(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) != "[[hooks.PreToolUse]]" {
			kept = append(kept, lines[i])
			i++
			continue
		}

		start := i
		end := i + 1
		hasHook := false
		for end < len(lines) {
			trimmed := strings.TrimSpace(lines[end])
			if strings.HasPrefix(trimmed, "[") && trimmed != "[[hooks.PreToolUse.hooks]]" {
				break
			}
			if trimmed == "[[hooks.PreToolUse.hooks]]" {
				hasHook = true
			}
			end++
		}
		if hasHook {
			kept = append(kept, lines[start:end]...)
		}
		i = end
	}

	result := strings.TrimRight(strings.Join(kept, "\n"), "\r\n")
	if result != "" {
		result += "\n"
	}
	return result
}

func setupOpencode(home string, remove bool) (string, error) {
	dir := filepath.Join(home, ".config", "opencode")
	pluginPath := filepath.Join(dir, "plugins", "hef.ts")
	configPath := filepath.Join(dir, "opencode.json")
	entry := "./plugins/hef.ts"
	legacyPluginPath := filepath.Join(dir, "plugins", "hef-guard.ts")
	legacyEntry := "C:/Users/young/.config/opencode/plugins/hef-guard.ts"

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
	plugins = removeString(plugins, legacyEntry)
	if !containsString(plugins, entry) {
		plugins = append(plugins, entry)
	}
	config["plugin"] = plugins
	if err := writeJSON(configPath, config); err != nil {
		return "", err
	}
	if err := removeFileIfExists(legacyPluginPath); err != nil {
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

function Get-PropertyValue {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property) { return $null }
    return $property.Value
}

function Add-CommandTexts {
    param($Object, [System.Collections.Generic.List[string]]$Commands)
    if ($null -eq $Object) { return }

    $command = Get-PropertyValue -Object $Object -Name "command"
    if ($command -is [string]) { $Commands.Add($command) }

    foreach ($name in @("tool_input", "parameters")) {
        $child = Get-PropertyValue -Object $Object -Name $name
        if ($null -ne $child) { Add-CommandTexts -Object $child -Commands $Commands }
    }

    $toolUses = Get-PropertyValue -Object $Object -Name "tool_uses"
    if ($null -ne $toolUses) {
        foreach ($toolUse in @($toolUses)) { Add-CommandTexts -Object $toolUse -Commands $Commands }
    }
}

$toolInput = Get-PropertyValue -Object $hookInput -Name "tool_input"
if ($null -ne $toolInput) {
    $backgroundProperty = $toolInput.PSObject.Properties["run_in_background"]
    if ($null -ne $backgroundProperty -and $backgroundProperty.Value -eq $true) {
        exit 0
    }
}

$commands = New-Object 'System.Collections.Generic.List[string]'
Add-CommandTexts -Object $hookInput -Commands $commands
if ($commands.Count -eq 0) {
    exit 0
}

foreach ($command in $commands) {
    if ([string]::IsNullOrWhiteSpace($command)) {
        continue
    }

    if ($command -match "(?i)(^|[\s;&|])hef(?:\.exe)?(\s|$)" -or
        $command -match "(?i)(\[no-hef\]|--no-hef|#\s*no-hef)") {
        continue
    }

    $reason = "긴 출력이 예상되는 셸 명령에는 반드시 human-eye-filter(hef)를 사용하세요: <command> | hef" +
        [Environment]::NewLine + "hef를 적용할 수 없는 특수한 경우에만 명령에 # no-hef를 명시하고 다시 실행하세요."
    $output = @{
        hookSpecificOutput = @{
            hookEventName = "PreToolUse"
            permissionDecision = "deny"
            permissionDecisionReason = $reason
        }
    }

    [Console]::Out.WriteLine(($output | ConvertTo-Json -Compress))
    exit 0
}

exit 0
`
}

func opencodePluginSource() string {
	return `import type { Plugin } from "@opencode-ai/plugin"

const HEF_COMMAND_PATTERN = /(^|[\s;&|])hef(?:\.exe)?(\s|$)/i
const HEF_OPT_OUT_PATTERN = /(\[no-hef\]|--no-hef|#\s*no-hef)/i

export const hef: Plugin = async () => {
  return {
    "tool.execute.before": async (input, output) => {
      const tool = input.tool
      if (tool !== "bash" && tool !== "shell") return

      const args = output.args ?? {}
      const command = args.command
      const description = args.description
      if (typeof command !== "string" || command.trim().length === 0) return
      if (HEF_COMMAND_PATTERN.test(command)) return
      if (HEF_OPT_OUT_PATTERN.test(command)) return
      if (typeof description === "string" && HEF_OPT_OUT_PATTERN.test(description)) return

      throw new Error(
        "[hef] 긴 출력이 예상되는 셸 명령에는 반드시 human-eye-filter(hef)를 사용하세요: '<command> | hef'. " +
        "hef를 적용할 수 없는 특수한 경우에만 description에 '[no-hef]' 또는 명령에 '# no-hef'를 명시하세요."
      )
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
