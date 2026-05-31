package humaneye

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
var numberPattern = regexp.MustCompile(`\d+`)
var pathLinePattern = regexp.MustCompile(`^(.+?):(\d+)(?::(\d+))?:(.*)$`)
var longTokenPattern = regexp.MustCompile(`(?i)(?:[A-Za-z]:[\\/]|https?://|[./\\])[^\s\]})>,;'"]{24,}|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

type filterStep struct {
	name string
	fn   func(string, Options) (string, bool)
}

type matchLine struct {
	line string
	text string
}

type numericPart struct {
	prefix string
	suffix string
	raw    string
	value  int
	width  int
}

type rangeRun struct {
	start numericPart
	end   numericPart
	count int
}

func Reduce(input string, options Options) (string, error) {
	input = normalizeInput(input)
	if strings.TrimSpace(input) == "" {
		return "", nil
	}

	current := input
	applied := []string{}
	for _, step := range []filterStep{
		{name: "RepeatedLine", fn: repeatedLineFilter},
		{name: "PathLineGroup", fn: pathLineGroupFilter},
		{name: "DirectoryGroup", fn: directoryGroupFilter},
		{name: "SequentialRange", fn: sequentialRangeFilter},
		{name: "CommonPrefix", fn: commonPrefixFilter},
		{name: "DictionaryToken", fn: dictionaryTokenFilter},
	} {
		next, ok := step.fn(current, options)
		if !ok || !shorter(current, next) {
			continue
		}
		current = next
		applied = append(applied, step.name)
	}

	if next, ok := boundedSampleFilter(current, options); ok {
		current = next
		applied = append(applied, "BoundedSample")
	}
	if len(applied) == 0 {
		applied = append(applied, "None")
	}

	return current + summary(input, current, applied), nil
}

func normalizeInput(input string) string {
	input = strings.TrimPrefix(input, "\ufeff")
	input = ansiEscapePattern.ReplaceAllString(input, "")
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	input = strings.ReplaceAll(input, "\x00", "")
	return strings.TrimRight(input, "\n")
}

func repeatedLineFilter(input string, options Options) (string, bool) {
	lines := linesOf(input)
	if len(lines) < 3 {
		return "", false
	}
	out := make([]string, 0, len(lines))
	changed := false
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && lines[j] == lines[i] {
			j++
		}
		count := j - i
		if count >= 3 {
			out = append(out, fmt.Sprintf("%s (x%d)", lines[i], count))
			changed = true
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	return strings.Join(out, "\n"), changed
}

func sequentialRangeFilter(input string, options Options) (string, bool) {
	lines := linesOf(input)
	if len(lines) < 3 {
		return "", false
	}
	out := make([]string, 0, len(lines))
	changed := false
	for i := 0; i < len(lines); {
		run := findSequentialRun(lines, i)
		if run.count >= 3 {
			out = append(out, formatRange(run))
			changed = true
			i += run.count
			continue
		}
		out = append(out, lines[i])
		i++
	}
	return strings.Join(out, "\n"), changed
}

func pathLineGroupFilter(input string, options Options) (string, bool) {
	lines := linesOf(input)
	if len(lines) < 3 {
		return "", false
	}
	groups := map[string][]matchLine{}
	seenByPath := map[string]map[string]bool{}
	order := []string{}
	unmatched := []string{}
	matches := 0
	for _, line := range lines {
		parts := pathLinePattern.FindStringSubmatch(line)
		if len(parts) == 0 || !looksPathish(parts[1]) {
			unmatched = append(unmatched, line)
			continue
		}
		path := strings.ReplaceAll(parts[1], "\\", "/")
		loc := parts[2]
		if parts[3] != "" {
			loc += ":" + parts[3]
		}
		if _, exists := groups[path]; !exists {
			order = append(order, path)
		}
		key := loc + "\x00" + strings.TrimSpace(parts[4])
		if seenByPath[path] == nil {
			seenByPath[path] = map[string]bool{}
		}
		if !seenByPath[path][key] {
			groups[path] = append(groups[path], matchLine{line: loc, text: strings.TrimSpace(parts[4])})
			seenByPath[path][key] = true
		}
		matches++
	}
	if matches < 3 || matches < len(lines)/2 {
		return "", false
	}
	sort.Strings(order)
	root := commonPathRoot(order)
	out := []string{}
	if root != "" && len(root) >= 12 {
		out = append(out, "$root="+root)
	}
	for _, path := range order {
		display := path
		if root != "" && strings.HasPrefix(display, root) {
			display = "$root" + strings.TrimPrefix(display, root)
		}
		items := groups[path]
		out = append(out, fmt.Sprintf("%s (%d)", display, len(items)))
		for _, item := range takeMatchLines(items, 4) {
			out = append(out, fmt.Sprintf("  %s: %s", item.line, item.text))
		}
		if len(items) > 4 {
			out = append(out, fmt.Sprintf("  ... <%d more>", len(items)-4))
		}
	}
	if len(unmatched) > 0 && len(unmatched) <= 8 {
		out = append(out, "## unmatched")
		out = append(out, unmatched...)
	}
	return strings.Join(out, "\n"), true
}

func directoryGroupFilter(input string, options Options) (string, bool) {
	lines := nonBlank(linesOf(input))
	if len(lines) < 6 {
		return "", false
	}
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.ReplaceAll(strings.TrimSpace(line), "\\", "/")
		if !looksPathish(line) {
			return "", false
		}
		normalized = append(normalized, line)
	}
	root := commonPathRoot(normalized)
	if root == "" {
		return "", false
	}
	groups := map[string][]string{}
	dirs := []string{}
	for _, path := range normalized {
		dir, file := splitPath(path)
		if _, exists := groups[dir]; !exists {
			dirs = append(dirs, dir)
		}
		groups[dir] = append(groups[dir], file)
	}
	sort.Strings(dirs)
	out := []string{"$root=" + root}
	for _, dir := range dirs {
		display := dir
		if strings.HasPrefix(display, root) {
			display = "$root" + strings.TrimPrefix(display, root)
		}
		out = append(out, fmt.Sprintf("%s/ (%d)", strings.TrimRight(display, "/"), len(groups[dir])))
		files := append([]string{}, groups[dir]...)
		sort.Strings(files)
		for _, file := range take(files, 8) {
			out = append(out, "  "+file)
		}
		if len(files) > 8 {
			out = append(out, fmt.Sprintf("  ... <%d more>", len(files)-8))
		}
	}
	return strings.Join(out, "\n"), true
}
func commonPrefixFilter(input string, options Options) (string, bool) {
	lines := nonBlank(linesOf(input))
	if len(lines) < 3 {
		return "", false
	}
	prefix := trimPrefixBoundary(commonPrefix(lines))
	if len(prefix) < 20 {
		return "", false
	}
	out := []string{"$prefix1=" + prefix}
	for _, line := range linesOf(input) {
		out = append(out, strings.Replace(line, prefix, "$prefix1", 1))
	}
	return strings.Join(out, "\n"), true
}

func dictionaryTokenFilter(input string, options Options) (string, bool) {
	matches := longTokenPattern.FindAllString(input, -1)
	if len(matches) == 0 {
		return "", false
	}
	counts := map[string]int{}
	for _, match := range matches {
		counts[match]++
	}
	tokens := []string{}
	for token, count := range counts {
		if count >= 2 && len(token) >= 24 {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return "", false
	}
	sort.Slice(tokens, func(i, j int) bool { return len(tokens[i]) > len(tokens[j]) })
	if len(tokens) > 6 {
		tokens = tokens[:6]
	}
	out := input
	defs := []string{}
	for i, token := range tokens {
		name := fmt.Sprintf("$t%d", i+1)
		out = strings.ReplaceAll(out, token, name)
		defs = append(defs, name+"="+token)
	}
	return strings.Join(defs, "\n") + "\n" + out, true
}

func boundedSampleFilter(input string, options Options) (string, bool) {
	maxLines := options.MaxLines
	if maxLines <= 0 {
		maxLines = DefaultOptions().MaxLines
	}
	maxChars := options.MaxChars
	if maxChars <= 0 {
		maxChars = DefaultOptions().MaxChars
	}
	lines := linesOf(input)
	if len(lines) <= maxLines && len(input) <= maxChars {
		return "", false
	}
	important := importantLines(lines, focusKeywords(options))
	head := maxLines / 4
	if head < 6 {
		head = 6
	}
	tail := maxLines / 4
	if tail < 6 {
		tail = 6
	}
	out := []string{"## head"}
	out = append(out, take(lines, head)...)
	if len(important) > 0 {
		out = append(out, "## important")
		out = append(out, take(important, maxLines/3)...)
	}
	out = append(out, "## tail")
	out = append(out, takeTail(lines, tail)...)
	out = append(out, fmt.Sprintf("... <omitted %d lines>", omittedCount(len(lines), head, tail)))
	result := strings.Join(out, "\n")
	if len(result) > maxChars {
		result = result[:maxChars-32] + "\n... <char limit reached>"
	}
	return result, true
}

func findSequentialRun(lines []string, start int) rangeRun {
	best := rangeRun{count: 1}
	for _, first := range numericCandidates(lines[start]) {
		current := first
		count := 1
		for i := start + 1; i < len(lines); i++ {
			next, ok := matchingNumber(lines[i], current)
			if !ok || next.value != current.value+1 {
				break
			}
			current = next
			count++
		}
		if count > best.count {
			best = rangeRun{start: first, end: current, count: count}
		}
	}
	return best
}

func numericCandidates(line string) []numericPart {
	matches := numberPattern.FindAllStringIndex(line, -1)
	parts := make([]numericPart, 0, len(matches))
	for _, match := range matches {
		raw := line[match[0]:match[1]]
		value, err := strconv.Atoi(raw)
		if err != nil {
			continue
		}
		parts = append(parts, numericPart{prefix: line[:match[0]], suffix: line[match[1]:], raw: raw, value: value, width: len(raw)})
	}
	return parts
}

func matchingNumber(line string, previous numericPart) (numericPart, bool) {
	for _, candidate := range numericCandidates(line) {
		if candidate.prefix == previous.prefix && candidate.suffix == previous.suffix && candidate.width == previous.width {
			return candidate, true
		}
	}
	return numericPart{}, false
}

func formatRange(run rangeRun) string {
	end := run.end.raw
	if run.start.width > 1 {
		end = fmt.Sprintf("%0*d", run.start.width, run.end.value)
	}
	return fmt.Sprintf("%s%s..%s%s (%d lines)", run.start.prefix, run.start.raw, end, run.start.suffix, run.count)
}

func linesOf(input string) []string {
	if input == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(input, "\n"), "\n")
}

func takeMatchLines(values []matchLine, count int) []matchLine {
	if count < 0 {
		count = 0
	}
	if len(values) <= count {
		return append([]matchLine{}, values...)
	}
	return append([]matchLine{}, values[:count]...)
}

func looksPathish(value string) bool {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return false
	}
	return strings.Contains(value, "/") && (strings.Contains(lastPathPart(value), ".") || strings.Contains(value, ":/"))
}

func splitPath(path string) (string, string) {
	path = strings.TrimRight(path, "/")
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "", path
	}
	return path[:idx], path[idx+1:]
}

func lastPathPart(path string) string {
	_, file := splitPath(path)
	return file
}

func commonPathRoot(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	dirs := make([][]string, 0, len(paths))
	for _, path := range paths {
		dir, _ := splitPath(path)
		dirs = append(dirs, strings.Split(strings.Trim(dir, "/"), "/"))
	}
	if len(dirs) == 0 || len(dirs[0]) == 0 {
		return ""
	}
	common := []string{}
	for i, part := range dirs[0] {
		for _, dir := range dirs[1:] {
			if i >= len(dir) || dir[i] != part {
				return strings.Join(common, "/")
			}
		}
		common = append(common, part)
	}
	return strings.Join(common, "/")
}
func nonBlank(lines []string) []string {
	out := []string{}
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func commonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}
	prefix := values[0]
	for _, value := range values[1:] {
		for prefix != "" && !strings.HasPrefix(value, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func trimPrefixBoundary(prefix string) string {
	idx := strings.LastIndexAny(prefix, "/\\._:- ")
	if idx <= 0 {
		return prefix
	}
	return prefix[:idx+1]
}

func importantLines(lines []string, focus []string) []string {
	keys := append([]string{"error", "exception", "failed", "fatal", "denied", "warning", "panic"}, focus...)
	seen := map[string]bool{}
	out := []string{}
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, key := range keys {
			if key != "" && strings.Contains(lower, key) && !seen[line] {
				out = append(out, line)
				seen[line] = true
				break
			}
		}
	}
	return out
}

func focusKeywords(options Options) []string {
	parts := strings.Split(options.Focus, ",")
	out := []string{}
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func take(values []string, count int) []string {
	if count < 0 {
		count = 0
	}
	if len(values) <= count {
		return append([]string{}, values...)
	}
	return append([]string{}, values[:count]...)
}

func takeTail(values []string, count int) []string {
	if count < 0 {
		count = 0
	}
	if len(values) <= count {
		return append([]string{}, values...)
	}
	return append([]string{}, values[len(values)-count:]...)
}

func omittedCount(total int, head int, tail int) int {
	omitted := total - head - tail
	if omitted < 0 {
		return 0
	}
	return omitted
}

func shorter(before string, after string) bool {
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	return after != "" && after != before && len(after) < len(before)
}

func summary(input string, output string, filters []string) string {
	return fmt.Sprintf("\n\n--- hef summary ---\nfilters=%s lines=%d->%d chars=%d->%d\n",
		strings.Join(filters, "+"),
		len(linesOf(input)),
		len(linesOf(output)),
		len(input),
		len(output))
}
