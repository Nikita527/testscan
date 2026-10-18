package rules

import "strings"

// looksLikeMockName reports Mock/patch-style receivers: token-boundary
// mock/mocker, CamelCase *Mock*, or camelCase mockFoo — not hammock/demock.
func looksLikeMockName(root string) bool {
	rl := strings.ToLower(root)
	if hasMockToken(rl) || strings.Contains(root, "Mock") {
		return true
	}
	// camelCase mockFoo (prefix "mock" + uppercase).
	if len(root) > 4 && strings.HasPrefix(rl, "mock") && root[4] >= 'A' && root[4] <= 'Z' {
		return true
	}
	return false
}

// hasMockToken reports whether name contains "mock"/"mocks"/"mocker" as an
// identifier token (underscore-separated), not a substring of another word
// (e.g. hammock; mockery does not match "mock" mid-word).
func hasMockToken(name string) bool {
	return identHasToken(strings.ToLower(name), "mock") ||
		identHasToken(strings.ToLower(name), "mocks") ||
		identHasToken(strings.ToLower(name), "mocker")
}

// hasStubToken is the same idea for "stub" / "stubs".
func hasStubToken(name string) bool {
	return identHasToken(strings.ToLower(name), "stub") ||
		identHasToken(strings.ToLower(name), "stubs")
}

// identHasToken reports whether tok appears in s with underscore (or string)
// boundaries — not mid-word.
func identHasToken(s, tok string) bool {
	if s == "" || tok == "" {
		return false
	}
	start := 0
	for {
		i := strings.Index(s[start:], tok)
		if i < 0 {
			return false
		}
		i += start
		beforeOK := i == 0 || s[i-1] == '_'
		after := i + len(tok)
		afterOK := after >= len(s) || s[after] == '_'
		if beforeOK && afterOK {
			return true
		}
		start = i + 1
	}
}
