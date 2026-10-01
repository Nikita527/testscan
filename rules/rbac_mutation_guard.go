package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

var (
	defaultRBACNameCues = []string{
		"rbac", "permission", "forbidden", "role", "protected",
	}
	defaultMutatingMethods = []string{
		"post", "put", "patch", "delete",
		"create", "update", "destroy", "remove",
	}
	defaultForbiddenSignals = []string{
		"401", "403", "HTTP_401", "HTTP_403",
		"forbidden", "permissiondenied", "permission_denied",
		"PermissionDenied", "NotAuthenticated",
	}
	reHTTPMutating     = regexp.MustCompile(`(?i)\.(post|put|patch|delete)\(`)
	reDigitsOnlySignal = regexp.MustCompile(`^\d+$`)
)

// RBACMutationGuardOpts configures rbac-mutation-guard.
type RBACMutationGuardOpts struct {
	NameCues         []string
	MutatingMethods  []string
	ForbiddenSignals []string
}

type rbacMutationGuard struct {
	nameCues         []string
	mutatingMethods  []string
	forbiddenSignals []string
}

func (rbacMutationGuard) ID() string { return "rbac-mutation-guard" }

func (rbacMutationGuard) NeedsAST() bool { return true }

func (r rbacMutationGuard) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	cues := r.nameCues
	if len(cues) == 0 {
		cues = defaultRBACNameCues
	}
	mutating := r.mutatingMethods
	if len(mutating) == 0 {
		mutating = defaultMutatingMethods
	}
	forbidden := r.forbiddenSignals
	if len(forbidden) == 0 {
		forbidden = defaultForbiddenSignals
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		if !testLooksRBAC(t, cues) {
			continue
		}
		if !testHasMutatingAction(t, mutating) {
			continue
		}
		if testHasForbiddenSignal(t, forbidden) {
			continue
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     t.Lineno,
			Rule:     "rbac-mutation-guard",
			Severity: "warning",
			Message:  "RBAC/permission-style test performs a mutating action without asserting forbid (401/403 / PermissionDenied / forbidden)",
			QualName: qualName(t),
		})
	}
	return findings
}

func testLooksRBAC(t parse.TestFunc, cues []string) bool {
	// Underscore-token match on name/qualname/fixtures so short cues like "role"
	// do not fire on arbitrary substrings.
	parts := []string{t.Name, t.QualName}
	parts = append(parts, t.Fixtures...)
	padded := "_" + strings.ToLower(strings.Join(parts, "_")) + "_"
	decorBlob := strings.ToLower(strings.Join(t.Decorators, " "))
	for _, c := range cues {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if strings.Contains(padded, "_"+c+"_") {
			return true
		}
		// Decorators often use CamelCase / dotted paths — allow substring there.
		if decorBlob != "" && strings.Contains(decorBlob, c) {
			return true
		}
	}
	return false
}

func testHasMutatingAction(t parse.TestFunc, methods []string) bool {
	methodSet := make(map[string]struct{}, len(methods))
	for _, m := range methods {
		methodSet[strings.ToLower(strings.TrimSpace(m))] = struct{}{}
	}
	for _, c := range t.Calls {
		leaf := strings.ToLower(leafName(c.Name))
		if _, ok := methodSet[leaf]; ok {
			return true
		}
		// client.post / api.delete
		parts := strings.Split(strings.ToLower(c.Name), ".")
		if len(parts) >= 2 {
			if _, ok := methodSet[parts[len(parts)-1]]; ok {
				return true
			}
		}
	}
	return reHTTPMutating.MatchString(t.BodyNorm)
}

func testHasForbiddenSignal(t parse.TestFunc, signals []string) bool {
	if t.HasRaises || len(t.Raises) > 0 {
		for _, r := range t.Raises {
			el := strings.ToLower(r.Exc)
			for _, s := range signals {
				sl := strings.ToLower(s)
				if strings.Contains(el, sl) || strings.Contains(el, "permission") {
					return true
				}
			}
		}
		// any raises may be intentional forbid check
		for _, r := range t.Raises {
			el := strings.ToLower(r.Exc)
			if strings.Contains(el, "permission") || strings.Contains(el, "forbidden") ||
				strings.Contains(el, "auth") {
				return true
			}
		}
	}
	var blob strings.Builder
	blob.WriteString(strings.ToLower(t.BodyNorm))
	for _, a := range t.Asserts {
		blob.WriteString(" ")
		blob.WriteString(strings.ToLower(a.Text + " " + a.Left + " " + a.Right))
	}
	text := blob.String()
	for _, s := range signals {
		sl := strings.ToLower(strings.TrimSpace(s))
		if sl == "" {
			continue
		}
		if forbiddenSignalInText(text, sl) {
			return true
		}
	}
	return false
}

// forbiddenSignalInText matches numeric HTTP codes on digit boundaries (not "1401")
// and other signals as plain substrings.
func forbiddenSignalInText(text, signal string) bool {
	if reDigitsOnlySignal.MatchString(signal) {
		for i := 0; i+len(signal) <= len(text); i++ {
			if text[i:i+len(signal)] != signal {
				continue
			}
			leftOK := i == 0 || text[i-1] < '0' || text[i-1] > '9'
			right := i + len(signal)
			rightOK := right == len(text) || text[right] < '0' || text[right] > '9'
			if leftOK && rightOK {
				return true
			}
		}
		return false
	}
	return strings.Contains(text, signal)
}

func NewRBACMutationGuard(opts RBACMutationGuardOpts) scan.Rule {
	return rbacMutationGuard{
		nameCues:         append([]string{}, opts.NameCues...),
		mutatingMethods:  append([]string{}, opts.MutatingMethods...),
		forbiddenSignals: append([]string{}, opts.ForbiddenSignals...),
	}
}
