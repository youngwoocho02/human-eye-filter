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

// runInstall implements `humaneye hook install` and `humaneye hook uninstall`.
func runInstall(action string, args []string, stdout, stderr io.Writer) (int, error) {
	flags := flag.NewFlagSet("humaneye hook "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	agent := flags.String("agent", "all", "agent to configure: claude, codex, opencode, all")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, nil
		}
		return 2, err
	}

	var agents []string
	switch *agent {
	case "all":
		agents = []string{"claude", "codex", "opencode"}
	case "claude", "codex", "opencode":
		agents = []string{*agent}
	default:
		return 2, fmt.Errorf("unknown agent %q (want claude, codex, opencode, all)", *agent)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return 1, err
	}
	self, err := os.Executable()
	if err != nil {
		return 1, err
	}
	selfFwd := strings.ReplaceAll(self, "\\", "/")

	uninstall := action == "uninstall"
	for _, a := range agents {
		var summary string
		var aerr error
		switch a {
		case "claude":
			summary, aerr = applyClaude(home, selfFwd, uninstall)
		case "codex":
			summary, aerr = applyCodex(home, selfFwd, uninstall)
		case "opencode":
			summary, aerr = applyOpencode(home, selfFwd, uninstall)
		}
		if aerr != nil {
			return 1, fmt.Errorf("%s: %w", a, aerr)
		}
		if _, err := fmt.Fprintf(stdout, "%s: %s\n", a, summary); err != nil {
			return 1, err
		}
	}
	return 0, nil
}

// --- claude (settings.json) ---

func applyClaude(home, selfFwd string, uninstall bool) (string, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	reg := "\"" + selfFwd + "\" hook claude"

	v := map[string]interface{}{}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(strings.TrimSpace(string(raw))) > 0 {
			if jerr := json.Unmarshal(raw, &v); jerr != nil {
				return "", fmt.Errorf("parse %s: %w", path, jerr)
			}
		}
	case os.IsNotExist(err):
		// treat as {}
	default:
		return "", err
	}

	hooks, _ := v["hooks"].(map[string]interface{})
	preList, _ := hooks["PreToolUse"].([]interface{})

	// Find an existing PreToolUse entry whose command contains "hook claude".
	idx := -1
	for i, item := range preList {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		inner, ok := obj["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, h := range inner {
			hobj, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if cmd, ok := hobj["command"].(string); ok && strings.Contains(cmd, "hook claude") {
				idx = i
			}
		}
	}

	if uninstall {
		if idx < 0 {
			return "no change (hook absent)", nil
		}
		preList = append(preList[:idx], preList[idx+1:]...)
		if len(preList) == 0 {
			delete(hooks, "PreToolUse")
		} else {
			hooks["PreToolUse"] = preList
		}
		if len(hooks) == 0 {
			delete(v, "hooks")
		} else {
			v["hooks"] = hooks
		}
		if werr := writeJSON(path, v); werr != nil {
			return "", werr
		}
		return "removed PreToolUse hook from " + path, nil
	}

	entry := map[string]interface{}{
		"matcher": "Bash",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": reg,
			},
		},
	}
	if hooks == nil {
		hooks = map[string]interface{}{}
	}
	if idx >= 0 {
		preList[idx] = entry
	} else {
		preList = append(preList, entry)
	}
	hooks["PreToolUse"] = preList
	v["hooks"] = hooks
	if werr := writeJSON(path, v); werr != nil {
		return "", werr
	}
	if idx >= 0 {
		return "kept PreToolUse hook in " + path, nil
	}
	return "added PreToolUse hook to " + path, nil
}

func writeJSON(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// --- codex (config.toml) ---

func codexBlock(selfFwd string) string {
	return "[[hooks.PreToolUse]]\n\n[[hooks.PreToolUse.hooks]]\ntype = \"command\"\ncommand = '\"" + selfFwd + "\" hook codex'\ntimeout = 5\n"
}

func applyCodex(home, selfFwd string, uninstall bool) (string, error) {
	path := filepath.Join(home, ".codex", "config.toml")

	raw, err := os.ReadFile(path)
	exists := true
	if err != nil {
		if os.IsNotExist(err) {
			exists = false
			raw = nil
		} else {
			return "", err
		}
	}
	text := string(raw)
	present := strings.Contains(text, "hook codex")

	if uninstall {
		if !present {
			return "no change (hook absent)", nil
		}
		newText, ok := removeCodexBlock(text)
		if !ok {
			return "left file unchanged (could not cleanly remove block); manual edit needed at " + path, nil
		}
		if werr := os.WriteFile(path, []byte(newText), 0o644); werr != nil {
			return "", werr
		}
		return "removed PreToolUse block from " + path, nil
	}

	if present {
		return "no change (hook already present)", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	block := codexBlock(selfFwd)
	if exists && len(text) > 0 && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if exists && len(text) > 0 {
		text += "\n"
	}
	text += block
	if werr := os.WriteFile(path, []byte(text), 0o644); werr != nil {
		return "", werr
	}
	return "appended PreToolUse block to " + path, nil
}

// removeCodexBlock removes the `[[hooks.PreToolUse]]` block that contains the
// `hook codex` command line. Returns (newText, true) on a clean removal.
func removeCodexBlock(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	// Locate the line carrying the hook codex command.
	cmdLine := -1
	for i, ln := range lines {
		if strings.Contains(ln, "hook codex") {
			cmdLine = i
			break
		}
	}
	if cmdLine < 0 {
		return text, false
	}
	// Walk backwards to the opening [[hooks.PreToolUse]] header.
	start := -1
	for i := cmdLine; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "[[hooks.PreToolUse]]" {
			start = i
			break
		}
	}
	if start < 0 {
		return text, false
	}
	// Walk forward from cmdLine to the end of the block: next table header or EOF.
	end := len(lines)
	for i := cmdLine + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "[") {
			end = i
			break
		}
	}
	// Trim leading blank lines that preceded the block.
	for start > 0 && strings.TrimSpace(lines[start-1]) == "" {
		start--
	}
	kept := append([]string{}, lines[:start]...)
	kept = append(kept, lines[end:]...)
	result := strings.Join(kept, "\n")
	result = strings.TrimRight(result, "\n")
	if result != "" {
		result += "\n"
	}
	return result, true
}

// --- opencode (plugins/humaneye.ts + opencode.json) ---

const opencodePluginEntry = "./plugins/humaneye.ts"

func opencodePluginSource(selfFwd string) string {
	return `import type { Plugin } from "@opencode-ai/plugin"

const SELF = "` + selfFwd + `"

export const humaneye: Plugin = async () => {
  return {
    "tool.execute.before": async (input, output) => {
      if (input.tool !== "bash" && input.tool !== "shell") return
      const command = output.args?.command
      if (typeof command !== "string" || command.length === 0) return
      if (command.includes("humaneye")) return
      const escaped = command.replace(/'/g, "'\\''")
      output.args.command = '"' + SELF + '" run-auto -- bash -c ' + "'" + escaped + "'"
    },
  }
}

export default humaneye
`
}

func applyOpencode(home, selfFwd string, uninstall bool) (string, error) {
	dir := filepath.Join(home, ".config", "opencode")
	pluginPath := filepath.Join(dir, "plugins", "humaneye.ts")
	jsonPath := filepath.Join(dir, "opencode.json")

	if uninstall {
		changed := false
		var notes []string

		raw, err := os.ReadFile(jsonPath)
		if err == nil {
			v := map[string]interface{}{}
			if len(strings.TrimSpace(string(raw))) > 0 {
				if jerr := json.Unmarshal(raw, &v); jerr != nil {
					return "", fmt.Errorf("parse %s: %w", jsonPath, jerr)
				}
			}
			if list, ok := v["plugin"].([]interface{}); ok {
				kept := make([]interface{}, 0, len(list))
				for _, p := range list {
					if s, ok := p.(string); ok && s == opencodePluginEntry {
						continue
					}
					kept = append(kept, p)
				}
				if len(kept) != len(list) {
					changed = true
					if len(kept) == 0 {
						delete(v, "plugin")
					} else {
						v["plugin"] = kept
					}
					if werr := writeJSON(jsonPath, v); werr != nil {
						return "", werr
					}
					notes = append(notes, "removed plugin entry from "+jsonPath)
				}
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}

		if _, serr := os.Stat(pluginPath); serr == nil {
			if rerr := os.Remove(pluginPath); rerr != nil {
				return "", rerr
			}
			changed = true
			notes = append(notes, "deleted "+pluginPath)
		}

		if !changed {
			return "no change (plugin absent)", nil
		}
		return strings.Join(notes, "; "), nil
	}

	// install
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		return "", err
	}
	if werr := os.WriteFile(pluginPath, []byte(opencodePluginSource(selfFwd)), 0o644); werr != nil {
		return "", werr
	}

	v := map[string]interface{}{}
	raw, err := os.ReadFile(jsonPath)
	switch {
	case err == nil:
		if len(strings.TrimSpace(string(raw))) > 0 {
			if jerr := json.Unmarshal(raw, &v); jerr != nil {
				return "", fmt.Errorf("parse %s: %w", jsonPath, jerr)
			}
		}
	case os.IsNotExist(err):
	default:
		return "", err
	}

	list, _ := v["plugin"].([]interface{})
	found := false
	for _, p := range list {
		if s, ok := p.(string); ok && s == opencodePluginEntry {
			found = true
			break
		}
	}
	if !found {
		list = append(list, opencodePluginEntry)
		v["plugin"] = list
	} else {
		v["plugin"] = list
	}
	if werr := writeJSON(jsonPath, v); werr != nil {
		return "", werr
	}
	if found {
		return "wrote plugin; entry already in " + jsonPath, nil
	}
	return "wrote plugin and added entry to " + jsonPath, nil
}
