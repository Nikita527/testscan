package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
)

// Lock-patch detection helpers for overmocked-lock (kept out of ast.go).

// lockPatchNeedles are locking-API leaf substrings (Django / SQLAlchemy).
// select_for_update also matches select_for_update_of.
var lockPatchNeedles = []string{
	"select_for_update",
	"with_for_update",
	"with_lockmode",
}

func hasLockPatchNeedle(s string) bool {
	for _, n := range lockPatchNeedles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// stripPythonComment removes a #-comment outside quotes (best-effort).
func stripPythonComment(line string) string {
	var inStr byte
	escape := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inStr != 0 {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == inStr {
				inStr = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inStr = c
			continue
		}
		if c == '#' {
			return line[:i]
		}
	}
	return line
}

func lockScanLine(line string) string {
	return strings.TrimSpace(stripPythonComment(line))
}

// lineLooksLikePatchCall is true for patch / monkeypatch.setattr / setattr call sites.
// Uses "patch(" (not bare "patch") so names like dispatch do not match.
// Bare "monkeypatch" alone is not enough (avoids comment FPs).
func lineLooksLikePatchCall(t string) bool {
	t = lockScanLine(t)
	if t == "" {
		return false
	}
	return strings.Contains(t, "patch(") ||
		strings.Contains(t, "patch.object(") ||
		strings.Contains(t, "monkeypatch.setattr") ||
		strings.Contains(t, "setattr(") ||
		strings.Contains(t, "@patch")
}

func lineLooksLikeSetattrCall(t string) bool {
	t = lockScanLine(t)
	return strings.Contains(t, "setattr(") || strings.Contains(t, "monkeypatch.setattr")
}

// isLockPatchingCall reports patch / patch.object / setattr / monkeypatch.setattr.
func isLockPatchingCall(name string) bool {
	if name == "patch" || name == "patch.object" || name == "setattr" {
		return true
	}
	leaf := leafName(name)
	if leaf == "setattr" || leaf == "patch" {
		return true
	}
	return strings.HasSuffix(name, ".patch.object") || strings.Contains(name, "patch.object")
}

func isSetattrLikeCall(name string) bool {
	return name == "setattr" || leafName(name) == "setattr"
}

// callParenSpanEnd returns the exclusive line index of the call that starts at
// startIdx, by matching parentheses (strings ignored roughly). limitIdx is exclusive.
func callParenSpanEnd(lines []string, startIdx, limitIdx int) int {
	if startIdx < 0 {
		startIdx = 0
	}
	if limitIdx > len(lines) {
		limitIdx = len(lines)
	}
	if startIdx >= limitIdx {
		return limitIdx
	}
	depth := 0
	started := false
	var inStr byte
	escape := false
	for i := startIdx; i < limitIdx; i++ {
		line := stripPythonComment(lines[i])
		for j := 0; j < len(line); j++ {
			c := line[j]
			if inStr != 0 {
				if escape {
					escape = false
					continue
				}
				if c == '\\' {
					escape = true
					continue
				}
				if c == inStr {
					inStr = 0
				}
				continue
			}
			if c == '"' || c == '\'' {
				inStr = c
				continue
			}
			switch c {
			case '(':
				depth++
				started = true
			case ')':
				if !started {
					continue
				}
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}
	}
	if started {
		return limitIdx
	}
	return startIdx + 1
}

// lastCallArg returns the last top-level argument inside the first call's (...).
func lastCallArg(span string) string {
	start := strings.Index(span, "(")
	if start < 0 {
		return ""
	}
	depth := 0
	var inStr byte
	escape := false
	end := -1
	for i := start; i < len(span); i++ {
		c := span[i]
		if inStr != 0 {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == inStr {
				inStr = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inStr = c
			continue
		}
		switch c {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end <= start+1 {
		return ""
	}
	args := splitTopLevelArgs(span[start+1 : end])
	if len(args) == 0 {
		return ""
	}
	return strings.TrimSpace(args[len(args)-1])
}

func splitTopLevelArgs(inner string) []string {
	var args []string
	depth := 0
	var inStr byte
	escape := false
	start := 0
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if inStr != 0 {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == inStr {
				inStr = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inStr = c
			continue
		}
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[start:i]))
				start = i + 1
			}
		}
	}
	tail := strings.TrimSpace(inner[start:])
	if tail != "" {
		args = append(args, tail)
	}
	return args
}

func spanHasMockFactoryLiteral(span string) bool {
	return strings.Contains(span, "MagicMock") ||
		strings.Contains(span, "AsyncMock") ||
		strings.Contains(span, "NonCallableMock") ||
		strings.Contains(span, "create_autospec") ||
		strings.Contains(span, " Mock(") ||
		strings.Contains(span, "= Mock(") ||
		strings.Contains(span, "(Mock(") ||
		strings.Contains(span, ", Mock(")
}

func nameUsedInMockAssert(t parse.TestFunc, name string) bool {
	if name == "" {
		return false
	}
	for _, a := range t.Asserts {
		if a.Kind != "mock_method" {
			continue
		}
		if strings.HasPrefix(a.Text, name+".") {
			return true
		}
		for _, r := range a.Reads {
			if r == name {
				return true
			}
		}
	}
	for _, c := range t.Calls {
		leaf := leafName(c.Name)
		if !strings.HasPrefix(leaf, "assert_called") &&
			leaf != "assert_has_calls" &&
			leaf != "assert_any_call" &&
			leaf != "assert_not_called" {
			continue
		}
		if c.Name == name+"."+leaf || strings.HasPrefix(c.Name, name+".") {
			return true
		}
	}
	return false
}

func identFromArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if i := strings.Index(arg, "#"); i >= 0 {
		arg = strings.TrimSpace(arg[:i])
	}
	if i := strings.IndexAny(arg, "(["); i >= 0 {
		arg = strings.TrimSpace(arg[:i])
	}
	return arg
}

func lockSetattrSpanValueIsMock(span string, t parse.TestFunc) bool {
	if spanHasMockFactoryLiteral(span) {
		return true
	}
	arg := identFromArg(lastCallArg(span))
	if arg == "" {
		return false
	}
	if strings.HasPrefix(arg, "mock_") {
		return true
	}
	if origin, ok := t.VarOrigins[arg]; ok && isMockFactoryOrigin(origin) {
		return true
	}
	return nameUsedInMockAssert(t, arg)
}

func lockPatchSpan(lines []string, start, end int) string {
	parts := make([]string, 0, end-start)
	for i := start; i < end && i < len(lines); i++ {
		if s := lockScanLine(lines[i]); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}

// lockPatchInContentWithAssertHints scans for lock patches. assertSrc, when set,
// allows setattr value names that appear in mock asserts elsewhere in assertSrc.
func lockPatchInContentWithAssertHints(src, assertSrc string) bool {
	lines := strings.Split(src, "\n")
	for _, line := range lines {
		t := lockScanLine(line)
		if !lineLooksLikePatchCall(t) || !hasLockPatchNeedle(t) {
			continue
		}
		if lineLooksLikeSetattrCall(t) {
			if lockSetattrValueOKHeuristic(t, assertSrc) {
				return true
			}
			continue
		}
		return true
	}
	// Multiline with patch(\n  "...select_for_update"\n)
	for i, line := range lines {
		t := lockScanLine(line)
		if !lineLooksLikePatchCall(t) || !strings.Contains(t, "(") {
			continue
		}
		end := callParenSpanEnd(lines, i, len(lines))
		span := lockPatchSpan(lines, i, end)
		if !hasLockPatchNeedle(span) {
			continue
		}
		if lineLooksLikeSetattrCall(t) {
			if lockSetattrValueOKHeuristic(span, assertSrc) {
				return true
			}
			continue
		}
		return true
	}
	return false
}

func lockSetattrValueOKHeuristic(span, assertSrc string) bool {
	if spanHasMockFactoryLiteral(span) {
		return true
	}
	arg := identFromArg(lastCallArg(span))
	if arg == "" {
		return false
	}
	if strings.HasPrefix(arg, "mock_") {
		return true
	}
	if assertSrc == "" {
		assertSrc = span
	}
	return strings.Contains(assertSrc, arg+".assert_called") ||
		strings.Contains(assertSrc, arg+".assert_has_calls") ||
		strings.Contains(assertSrc, arg+".assert_any_call") ||
		strings.Contains(assertSrc, arg+".assert_not_called")
}

func hasLockAttrAssign(line string) bool {
	for _, n := range lockPatchNeedles {
		if strings.Contains(line, "."+n+" =") || strings.Contains(line, "."+n+"=") {
			return true
		}
	}
	return false
}

func isMockFactoryOrigin(origin string) bool {
	switch leafName(origin) {
	case "MagicMock", "Mock", "AsyncMock", "NonCallableMock", "create_autospec", "patch":
		return true
	default:
		return false
	}
}

func lockAttrRHSIsMock(line string, t parse.TestFunc) bool {
	line = stripPythonComment(line)
	if spanHasMockFactoryLiteral(line) ||
		strings.Contains(line, "MagicMock") ||
		strings.Contains(line, "AsyncMock") ||
		strings.Contains(line, "NonCallableMock") ||
		strings.Contains(line, "create_autospec") {
		return true
	}
	eq := strings.Index(line, "=")
	if eq < 0 {
		return false
	}
	name := identFromArg(line[eq+1:])
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "mock_") {
		return true
	}
	if origin, ok := t.VarOrigins[name]; ok && isMockFactoryOrigin(origin) {
		return true
	}
	return nameUsedInMockAssert(t, name)
}

// lockAttrReplaceInContent detects `.select_for_update = mock` style (heuristic / no AST).
func lockAttrReplaceInContent(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		code := stripPythonComment(line)
		if !hasLockAttrAssign(code) {
			continue
		}
		eq := strings.Index(code, "=")
		if eq < 0 {
			continue
		}
		name := identFromArg(code[eq+1:])
		if name == "" {
			continue
		}
		if spanHasMockFactoryLiteral(code) ||
			strings.Contains(code, "MagicMock") ||
			strings.Contains(code, "AsyncMock") ||
			strings.HasPrefix(name, "mock_") {
			return true
		}
		if strings.Contains(src, name+".assert_called") ||
			strings.Contains(src, name+".assert_has_calls") ||
			strings.Contains(src, name+".assert_any_call") ||
			strings.Contains(src, name+".assert_not_called") {
			return true
		}
	}
	return false
}

func testHasLockAttrReplace(t parse.TestFunc, lines []string, start, end int) bool {
	for i := start; i < end; i++ {
		code := stripPythonComment(lines[i])
		if !hasLockAttrAssign(code) {
			continue
		}
		if lockAttrRHSIsMock(code, t) {
			return true
		}
	}
	return false
}

// testHasLockPatch is true when this test patches or replaces a locking API.
// Decorators must themselves mention patch + a lock leaf (unparsed full text).
// Body scan uses only [Lineno, EndLineno] — no lookback into neighboring tests.
// Patch/setattr calls use a parenthesis span so multiline with patch(...) hits.
func testHasLockPatch(t parse.TestFunc, src string) bool {
	for _, d := range t.Decorators {
		if strings.Contains(d, "patch") && hasLockPatchNeedle(d) {
			return true
		}
	}
	lines := strings.Split(src, "\n")
	start := t.Lineno - 1
	if start < 0 {
		start = 0
	}
	end := t.EndLineno
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	for _, c := range t.Calls {
		if !isLockPatchingCall(c.Name) {
			continue
		}
		callStart := c.Lineno - 1
		if callStart < start || callStart >= end {
			continue
		}
		spanEnd := callParenSpanEnd(lines, callStart, end)
		span := lockPatchSpan(lines, callStart, spanEnd)
		if !hasLockPatchNeedle(span) {
			continue
		}
		if isSetattrLikeCall(c.Name) {
			if lockSetattrSpanValueIsMock(span, t) {
				return true
			}
			continue
		}
		return true
	}
	body := strings.Join(lines[start:end], "\n")
	if lockPatchInContentWithAssertHints(body, body) {
		return true
	}
	return testHasLockAttrReplace(t, lines, start, end)
}
