package humaneye

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
var grepLinePattern = regexp.MustCompile(`^(.+?):(\d+)(?::(\d+))?:\s?(.*)$`)
var pathLikePattern = regexp.MustCompile(`(?i)(^|[A-Za-z]:[\\/]|[./\\])[\w .@~+\-()[\]{}:]+[\\/][\w .@~+\-()[\]{}:]+\.[A-Za-z0-9_]+$`)
var scmStatusLinePattern = regexp.MustCompile(`(?i)^\s*(?:\?\?|[MADRCU?!]|CH|CO|CO\+CH|LD|LM|MV|AD|PR|DE|RP)\b`)

type grepMatch struct {
	location string
	text     string
}

type stats struct {
	InputLines   int
	OutputLines  int
	InputChars   int
	OutputChars  int
	Mode         string
	SectionsKept []string
}

func Reduce(input string, options Options) (string, error) {
	input = strings.TrimPrefix(input, "\ufeff")
	input = normalizeInput(input)
	if strings.TrimSpace(input) == "" {
		return "", nil
	}

	mode := normalizeMode(options.Mode)
	if mode == "auto" {
		detected := detectMode(input)
		if detected == "json" {
			mode = detected
		} else {
			mode = modeFromTool(options.Tool)
		}
		if mode == "auto" {
			mode = detectMode(input)
		}
	}

	var body string
	var sections []string
	switch mode {
	case "json":
		body, sections = reduceJSON(input, options)
	case "grep":
		body, sections = reduceGrep(input, options)
	case "paths":
		body, sections = reducePaths(input, options)
	case "unity":
		body, sections = reduceKeywordText(input, options, unityKeywords())
	case "scm":
		body, sections = reduceSCM(input, options)
	default:
		body, sections = reduceText(input, options)
	}

	body = enforceLimits(body, options)
	s := stats{
		InputLines:   countLines(input),
		OutputLines:  countLines(body),
		InputChars:   len(input),
		OutputChars:  len(body),
		Mode:         mode,
		SectionsKept: sections,
	}

	return body + formatSummary(s), nil
}

func modeFromTool(tool string) string {
	normalized := strings.ToLower(strings.TrimSpace(tool))
	normalized = strings.TrimSuffix(normalized, ".exe")
	switch normalized {
	case "", "auto":
		return "auto"
	case "rg", "ripgrep", "grep", "ag", "ack", "pt":
		return "grep"
	case "find", "fd":
		return "paths"
	case "git", "cm", "svn", "hg", "jj", "plastic":
		return "scm"
	case "unity", "unity-cli", "unity-scanner", "dotnet", "msbuild", "npm", "pnpm", "yarn", "go", "cargo", "rustc", "javac", "mvn", "gradle":
		return "unity"
	case "jq", "curl", "gh", "gh-api":
		return "json"
	default:
		return "auto"
	}
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "json", "paths", "grep", "unity", "scm", "text", "auto":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "auto"
	}
}

func detectMode(input string) string {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var value any
		if json.Unmarshal([]byte(trimmed), &value) == nil {
			return "json"
		}
	}

	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return "text"
	}

	grepHits := 0
	pathHits := 0
	unityHits := 0
	scmHits := 0
	for _, line := range sample(lines, 80) {
		if grepLinePattern.MatchString(line) {
			grepHits++
		}
		if pathLikePattern.MatchString(strings.TrimSpace(line)) {
			pathHits++
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "unity") || strings.Contains(lower, "exception") || strings.Contains(lower, "stacktrace") || strings.Contains(lower, "shader") {
			unityHits++
		}
		if strings.Contains(lower, "changeset") || strings.Contains(lower, "modified items") || strings.Contains(lower, "branch=") || strings.Contains(lower, "git status") || strings.Contains(lower, "changed") {
			scmHits++
		}
	}

	if grepHits >= 3 || grepHits*2 >= len(lines) {
		return "grep"
	}
	if pathHits >= 5 && pathHits*2 >= len(lines) {
		return "paths"
	}
	if unityHits >= 3 {
		return "unity"
	}
	if scmHits >= 2 {
		return "scm"
	}
	return "text"
}

func reduceText(input string, options Options) (string, []string) {
	lines := nonEmptyLines(input)
	lines = collapseDuplicateLines(lines)
	return strings.Join(prioritizeLines(lines, options), "\n"), []string{"deduped text"}
}

func reduceKeywordText(input string, options Options, keywords []string) (string, []string) {
	lines := nonEmptyLines(input)
	lines = collapseDuplicateLines(lines)
	priority, rest := splitPriorityLinesWithContext(lines, append(focusKeywords(options), keywords...))

	limit := options.MaxLines
	if limit <= 0 {
		limit = DefaultOptions().MaxLines
	}

	out := make([]string, 0, limit)
	if len(priority) > 0 {
		out = append(out, "## priority")
		out = append(out, take(priority, limit/2)...)
	}

	remaining := limit - len(out)
	if remaining > 1 && len(rest) > 0 {
		if len(out) > 0 {
			out = append(out, "## sample")
			remaining--
		}
		out = append(out, take(rest, remaining)...)
	}

	return strings.Join(out, "\n"), []string{"priority lines", "deduped sample"}
}

func reduceGrep(input string, options Options) (string, []string) {
	lines := nonEmptyLines(input)
	grouped := map[string][]grepMatch{}
	other := []string{}
	for _, line := range lines {
		matches := grepLinePattern.FindStringSubmatch(line)
		if len(matches) == 0 {
			other = append(other, line)
			continue
		}
		location := matches[2]
		if matches[3] != "" {
			location += ":" + matches[3]
		}
		grouped[matches[1]] = append(grouped[matches[1]], grepMatch{location: location, text: matches[4]})
	}

	paths := make([]string, 0, len(grouped))
	for path := range grouped {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	root := commonPathPrefix(paths)

	out := []string{}
	if root != "" {
		out = append(out, "$root="+root)
	}
	perFile := 4
	for _, path := range paths {
		items := grouped[path]
		out = append(out, fmt.Sprintf("%s (%d)", displayPath(path, root), len(items)))
		for _, item := range takeMatches(items, perFile) {
			out = append(out, fmt.Sprintf("  %s: %s", item.location, item.text))
		}
		if len(items) > perFile {
			out = append(out, fmt.Sprintf("  ... %d more", len(items)-perFile))
		}
	}

	if len(other) > 0 {
		out = append(out, "## unmatched")
		out = append(out, take(collapseDuplicateLines(other), 20)...)
	}

	return strings.Join(out, "\n"), []string{"grouped grep matches"}
}

func reducePaths(input string, options Options) (string, []string) {
	lines := collapseDuplicateLines(nonEmptyLines(input))
	tree := map[string][]string{}
	originalDirs := []string{}
	for _, line := range lines {
		normalized := strings.ReplaceAll(strings.TrimSpace(line), "\\", "/")
		if !pathLikePattern.MatchString(normalized) {
			continue
		}
		dir, file := splitPath(normalized)
		tree[dir] = append(tree[dir], file)
		originalDirs = append(originalDirs, dir)
	}

	dirs := make([]string, 0, len(tree))
	for dir := range tree {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	root := commonPathPrefix(originalDirs)

	out := []string{}
	if root != "" {
		out = append(out, "$root="+root)
	}
	for _, dir := range dirs {
		files := tree[dir]
		sort.Strings(files)
		out = append(out, fmt.Sprintf("%s/ (%d)", displayPath(dir, root), len(files)))
		for _, file := range take(files, 8) {
			out = append(out, "  "+file)
		}
		if len(files) > 8 {
			out = append(out, fmt.Sprintf("  ... %d more", len(files)-8))
		}
	}

	return strings.Join(out, "\n"), []string{"grouped paths"}
}

func reduceSCM(input string, options Options) (string, []string) {
	lines := collapseDuplicateLines(nonEmptyLines(input))
	focus := focusKeywords(options)
	sections := map[string][]string{
		"conflicts":      {},
		"content change": {},
		"checkout only":  {},
		"added":          {},
		"deleted/moved":  {},
		"branch/change":  {},
		"focused":        {},
		"other":          {},
	}
	order := []string{"conflicts", "content change", "checkout only", "added", "deleted/moved", "branch/change", "focused", "other"}

	for _, line := range lines {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "conflict") || strings.Contains(lower, "merge needed"):
			sections["conflicts"] = append(sections["conflicts"], line)
		case strings.HasPrefix(strings.TrimSpace(line), "CO+CH") || strings.HasPrefix(strings.TrimSpace(line), "CH"):
			sections["content change"] = append(sections["content change"], line)
		case strings.HasPrefix(strings.TrimSpace(line), "CO "):
			sections["checkout only"] = append(sections["checkout only"], line)
		case strings.HasPrefix(strings.TrimSpace(line), "AD") || strings.HasPrefix(strings.TrimSpace(line), "??") || strings.HasPrefix(strings.TrimSpace(line), "A "):
			sections["added"] = append(sections["added"], line)
		case strings.HasPrefix(strings.TrimSpace(line), "DE") || strings.HasPrefix(strings.TrimSpace(line), "LD") || strings.HasPrefix(strings.TrimSpace(line), "MV") || strings.HasPrefix(strings.TrimSpace(line), "D "):
			sections["deleted/moved"] = append(sections["deleted/moved"], line)
		case strings.Contains(lower, "branch") || strings.Contains(lower, "changeset") || strings.Contains(lower, "changelist"):
			sections["branch/change"] = append(sections["branch/change"], line)
		case isSCMStatusLine(line):
			sections["content change"] = append(sections["content change"], line)
		case containsAny(lower, focus):
			sections["focused"] = append(sections["focused"], line)
		default:
			sections["other"] = append(sections["other"], line)
		}
	}

	limit := options.MaxLines
	if limit <= 0 {
		limit = DefaultOptions().MaxLines
	}
	out := []string{}
	for _, name := range order {
		items := sections[name]
		if len(items) == 0 || len(out) >= limit {
			continue
		}
		remaining := limit - len(out)
		if remaining <= 1 {
			break
		}
		out = append(out, "## "+name)
		remaining--
		taken := take(items, remaining)
		out = append(out, taken...)
		if len(items) > len(taken) && len(out) < limit {
			out = append(out, fmt.Sprintf("... %d more %s lines", len(items)-len(taken), name))
		}
	}

	return strings.Join(out, "\n"), []string{"scm sections", "checkout/change split"}
}

func reduceJSON(input string, options Options) (string, []string) {
	var value any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &value); err != nil {
		return reduceText(input, options)
	}

	reduced := reduceJSONValue(value, 0)
	encoded, err := json.Marshal(reduced)
	if err != nil {
		return reduceText(input, options)
	}

	return string(encoded), []string{"sampled json"}
}

func reduceJSONValue(value any, depth int) any {
	if depth >= 4 {
		return "<depth limit>"
	}

	switch typed := value.(type) {
	case map[string]any:
		result := map[string]any{}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range take(keys, 24) {
			result[key] = reduceJSONValue(typed[key], depth+1)
		}
		if len(keys) > 24 {
			result["_humaneye_omitted_keys"] = len(keys) - 24
		}
		return result
	case []any:
		result := []any{}
		for _, item := range takeAny(typed, 8) {
			result = append(result, reduceJSONValue(item, depth+1))
		}
		if len(typed) > 8 {
			result = append(result, map[string]any{"_humaneye_omitted_items": len(typed) - 8})
		}
		return result
	case string:
		if runeLen(typed) > 240 {
			return truncateRunes(typed, 240) + "... <truncated " + strconv.Itoa(runeLen(typed)-240) + " chars>"
		}
		return typed
	default:
		return typed
	}
}

func splitPriorityLines(lines []string, keywords []string) ([]string, []string) {
	return splitPriorityLinesWithContext(lines, keywords)
}

func splitPriorityLinesWithContext(lines []string, keywords []string) ([]string, []string) {
	priority := []string{}
	rest := []string{}
	includeStackContext := false
	for _, line := range lines {
		lower := strings.ToLower(line)
		found := false
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				found = true
				break
			}
		}
		if found {
			priority = append(priority, line)
			includeStackContext = isErrorLike(lower) || isStackTraceStart(lower)
			continue
		}
		if isSCMStatusLine(line) {
			priority = append(priority, line)
			includeStackContext = false
			continue
		}
		if includeStackContext && isStackTraceLine(line) {
			priority = append(priority, line)
			continue
		}
		includeStackContext = false
		rest = append(rest, line)
	}
	return priority, rest
}

func isSCMStatusLine(line string) bool {
	return scmStatusLinePattern.MatchString(line)
}

func isErrorLike(lower string) bool {
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "exception") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "fatal")
}

func isStackTraceStart(lower string) bool {
	return strings.Contains(lower, "stacktrace") ||
		strings.Contains(lower, "stack trace")
}

func isStackTraceLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(trimmed, "at ") ||
		strings.Contains(trimmed, " at ") ||
		strings.HasPrefix(trimmed, "---") ||
		strings.Contains(lower, "stacktrace") ||
		strings.Contains(lower, "stack trace")
}

func prioritizeLines(lines []string, options Options) []string {
	if focus := focusKeywords(options); len(focus) > 0 {
		priority, rest := splitPriorityLines(lines, focus)
		lines = append(priority, rest...)
	}

	switch strings.ToLower(options.Keep) {
	case "all":
		return lines
	case "warning":
		priority, rest := splitPriorityLines(lines, []string{"warning", "warn"})
		return append(priority, rest...)
	case "path":
		priority, rest := splitPriorityLines(lines, []string{"/", "\\"})
		return append(priority, rest...)
	default:
		priority, rest := splitPriorityLines(lines, []string{"error", "exception", "failed", "fatal", "denied"})
		return append(priority, rest...)
	}
}

func focusKeywords(options Options) []string {
	parts := strings.Split(options.Focus, ",")
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(part))
		if trimmed == "" {
			continue
		}
		keywords = append(keywords, trimmed)
	}
	return keywords
}

func containsAny(value string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}

func normalizeInput(input string) string {
	input = strings.TrimPrefix(input, "\ufeff")
	input = ansiEscapePattern.ReplaceAllString(input, "")
	return strings.ReplaceAll(input, "\x00", "")
}

func unityKeywords() []string {
	return []string{"error", "exception", "failed", "stacktrace", "assert", "shader", "build failed", "compilation", " at ", "at ", "--- end of stack trace"}
}

func scmKeywords() []string {
	return []string{"error", "conflict", "modified", "changed", " ch ", " ch", "co+ch", "deleted", "added", "moved", "branch", "changeset", "pending", "created", "??", " m ", " d "}
}

func enforceLimits(input string, options Options) string {
	lines := strings.Split(strings.TrimRight(input, "\r\n"), "\n")
	maxLines := options.MaxLines
	if maxLines <= 0 {
		maxLines = DefaultOptions().MaxLines
	}
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], fmt.Sprintf("... <humaneye truncated %d lines>", len(lines)-maxLines))
	}

	output := strings.Join(lines, "\n")
	maxChars := options.MaxChars
	if maxChars <= 0 {
		maxChars = DefaultOptions().MaxChars
	}
	if runeLen(output) > maxChars {
		omitted := runeLen(output) - maxChars
		output = truncateRunes(output, maxChars) + fmt.Sprintf("\n... <humaneye truncated %d chars>", omitted)
	}

	return output
}

func runeLen(value string) int {
	return len([]rune(value))
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func formatSummary(s stats) string {
	sections := strings.Join(s.SectionsKept, ", ")
	if sections == "" {
		sections = "none"
	}
	return fmt.Sprintf("\n\n--- humaneye summary ---\nmode=%s lines=%d->%d chars=%d->%d kept=%s\n",
		s.Mode,
		s.InputLines,
		s.OutputLines,
		s.InputChars,
		s.OutputChars,
		sections)
}

func nonEmptyLines(input string) []string {
	raw := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		trimmed := strings.TrimPrefix(strings.TrimRight(line, " \t"), "\ufeff")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

func collapseDuplicateLines(lines []string) []string {
	out := []string{}
	var previous string
	count := 0
	flush := func() {
		if previous == "" {
			return
		}
		out = append(out, previous)
		if count > 1 {
			out = append(out, fmt.Sprintf("  <repeated %d times>", count))
		}
	}

	for _, line := range lines {
		if line == previous {
			count++
			continue
		}
		flush()
		previous = line
		count = 1
	}
	flush()
	return out
}

func countLines(input string) int {
	if strings.TrimSpace(input) == "" {
		return 0
	}
	return len(nonEmptyLines(input))
}

func sample(lines []string, max int) []string {
	if len(lines) <= max {
		return lines
	}
	return lines[:max]
}

func take[T any](items []T, max int) []T {
	if max < 0 {
		max = 0
	}
	if len(items) <= max {
		return items
	}
	return items[:max]
}

func takeAny(items []any, max int) []any {
	if len(items) <= max {
		return items
	}
	return items[:max]
}

func takeMatches(items []grepMatch, max int) []grepMatch {
	if len(items) <= max {
		return items
	}
	return items[:max]
}

func splitPath(path string) (string, string) {
	index := strings.LastIndex(path, "/")
	if index < 0 {
		return ".", path
	}
	return path[:index], path[index+1:]
}

func commonPathPrefix(paths []string) string {
	if len(paths) < 2 {
		return ""
	}

	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		normalized := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
		if normalized == "" {
			continue
		}
		dir, _ := splitPath(normalized)
		if dir == "." {
			dir = normalized
		}
		cleaned = append(cleaned, strings.TrimRight(dir, "/"))
	}
	if len(cleaned) < 2 {
		return ""
	}

	prefixParts := strings.Split(cleaned[0], "/")
	for _, path := range cleaned[1:] {
		parts := strings.Split(path, "/")
		max := len(prefixParts)
		if len(parts) < max {
			max = len(parts)
		}
		i := 0
		for i < max && prefixParts[i] == parts[i] {
			i++
		}
		prefixParts = prefixParts[:i]
		if len(prefixParts) == 0 {
			return ""
		}
	}

	root := strings.Join(prefixParts, "/")
	if len(root) < 24 || !strings.Contains(root, "/") {
		return ""
	}
	return root
}

func displayPath(path, root string) string {
	normalized := strings.ReplaceAll(path, "\\", "/")
	if root == "" {
		return normalized
	}
	if normalized == root {
		return "$root"
	}
	prefix := strings.TrimRight(root, "/") + "/"
	if strings.HasPrefix(normalized, prefix) {
		return "$root/" + strings.TrimPrefix(normalized, prefix)
	}
	return normalized
}
