package rules

import (
	"strings"
	"unicode"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type expectedRecomputed struct{}

func (expectedRecomputed) ID() string { return "expected-recomputed" }

func (expectedRecomputed) NeedsAST() bool { return true }

func (expectedRecomputed) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	src := string(file.Content)
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		seen := map[int]struct{}{}
		for _, a := range t.Asserts {
			if a.Kind != "compare" || !a.RightIsCall {
				continue
			}
			if _, dup := seen[a.Lineno]; dup {
				continue
			}
			rightLeaf := callLeafFromSide(a.Right)
			if rightLeaf == "" || isBuiltinOrLiteralCall(rightLeaf) || isTrivialMethodLeaf(rightLeaf) {
				continue
			}
			// Determinism: assert f(x) == f(x) — both sides are the same call; not a recompute smell.
			if a.LeftIsCall && strings.TrimSpace(a.Left) == strings.TrimSpace(a.Right) {
				continue
			}
			// Classic: got = compute(...); assert got == compute(...)
			left := strings.TrimSpace(a.Left)
			if !a.LeftIsCall && isSimpleIdent(left) &&
				nameAssignedFromLeaf(t, src, left, rightLeaf, a.Lineno) {
				seen[a.Lineno] = struct{}{}
				findings = append(findings, findingExpectedRecomputed(file.Path, a.Lineno, q))
			}
		}
	}
	return findings
}

func findingExpectedRecomputed(path string, line int, q string) scan.Finding {
	return scan.Finding{
		File:     path,
		Line:     line,
		Rule:     "expected-recomputed",
		Severity: "warning",
		Message:  "expected value recomputed via SUT/helper call on assert RHS",
		QualName: q,
	}
}

func isSimpleIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func isTrivialMethodLeaf(leaf string) bool {
	switch strings.ToLower(leaf) {
	case "index", "get", "keys", "values", "items", "lower", "upper", "strip",
		"lstrip", "rstrip", "split", "rsplit", "join", "format",
		"startswith", "endswith", "find", "rfind", "count", "append", "extend",
		"pop", "copy", "update", "encode", "decode", "bit_length",
		"isoformat", "timestamp", "astimezone":
		return true
	}
	return false
}

// nameAssignedFromLeaf reports whether `name = …leaf(` appears in the test
// before assertLine (got = compute(...); assert got == compute(...)).
func nameAssignedFromLeaf(t parse.TestFunc, src string, name, leaf string, assertLine int) bool {
	if src == "" || name == "" || leaf == "" {
		return false
	}
	lines := strings.Split(src, "\n")
	start := t.Lineno - 1
	if start < 0 {
		start = 0
	}
	end := assertLine - 1
	if end > len(lines) {
		end = len(lines)
	}
	if start >= end {
		return false
	}
	prefix := name + " ="
	prefix2 := name + "="
	for _, line := range lines[start:end] {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, prefix) && !strings.HasPrefix(trim, prefix2) {
			continue
		}
		// RHS of assignment should call leaf (compute( / mod.compute().
		eq := strings.Index(trim, "=")
		if eq < 0 {
			continue
		}
		rhs := trim[eq+1:]
		if callLeafFromSide(strings.TrimSpace(rhs)) == leaf {
			return true
		}
		if strings.Contains(rhs, "."+leaf+"(") || strings.Contains(rhs, leaf+"(") {
			return true
		}
	}
	return false
}

func callLeafFromSide(side string) string {
	side = strings.TrimSpace(side)
	if side == "" {
		return ""
	}
	// Strip trailing call args: compute(x) → compute, mod.fn(a, b) → fn
	if i := strings.Index(side, "("); i >= 0 {
		side = side[:i]
	}
	return leafName(side)
}

func isBuiltinOrLiteralCall(leaf string) bool {
	switch leaf {
	case "len", "str", "int", "float", "bool", "list", "dict", "set", "tuple",
		"sorted", "repr", "type", "isinstance", "getattr", "hasattr",
		"min", "max", "sum", "abs", "round", "enumerate", "zip", "map", "filter",
		"any", "all", "next", "iter", "range", "print", "id", "hash", "hex", "oct",
		"bytes", "bytearray", "memoryview", "complex", "object", "super",
		"frozenset", "slice", "format", "vars", "dir", "callable",
		// Common constructors / factories — not SUT recomputation.
		"datetime", "date", "time", "timedelta", "timezone",
		"UUID", "uuid4", "uuid1", "uuid3", "uuid5", "Decimal",
		"Path", "PurePath", "PosixPath", "WindowsPath",
		"defaultdict", "Counter", "OrderedDict", "namedtuple", "partial",
		"MagicMock", "Mock", "AsyncMock", "PropertyMock":
		return true
	}
	return false
}

func NewExpectedRecomputed() scan.Rule {
	return expectedRecomputed{}
}
