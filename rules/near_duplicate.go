package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type nearDuplicateTest struct{}

func (nearDuplicateTest) ID() string { return "near-duplicate-test" }

func (nearDuplicateTest) NeedsAST() bool { return true }

type nearDupMember struct {
	t        parse.TestFunc
	qual     string
	polarity string // "pos" | "neg" | ""
	sut      string
	lits     string // sorted significant assert literals
}

func (nearDuplicateTest) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}

	byNorm := map[string][]nearDupMember{}
	for _, t := range model.Tests {
		norm := t.BodyNorm
		if norm == "" || t.IsEmpty {
			continue
		}
		byNorm[norm] = append(byNorm[norm], nearDupMember{
			t:        t,
			qual:     qualName(t),
			polarity: nearDupPolarity(t),
			sut:      primarySUTCall(t),
			lits:     significantAssertLiterals(t),
		})
	}

	var findings []scan.Finding
	for _, group := range byNorm {
		if len(group) < 2 {
			continue
		}
		for _, c := range refineNearDupClusters(group) {
			if len(c) < 2 {
				continue
			}
			findings = append(findings, emitNearDupFindings(file.Path, c)...)
		}
	}
	return findings
}

func refineNearDupClusters(group []nearDupMember) [][]nearDupMember {
	n := len(group)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if nearDupCompatible(group[i], group[j]) {
				union(i, j)
			}
		}
	}

	buckets := map[int][]nearDupMember{}
	for i, m := range group {
		r := find(i)
		buckets[r] = append(buckets[r], m)
	}
	out := make([][]nearDupMember, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, b)
	}
	return out
}

func nearDupCompatible(a, b nearDupMember) bool {
	if a.qual == b.qual {
		return false
	}
	// Opposite polarity (accept ↔ reject, raises ↔ no, 2xx ↔ 4xx).
	if a.polarity != "" && b.polarity != "" && a.polarity != b.polarity {
		return false
	}
	// Different SUT calls.
	if a.sut != "" && b.sut != "" && a.sut != b.sut {
		return false
	}
	// Different meaningful enum / status expectations (BodyNorm collapses them).
	if a.lits != b.lits {
		return false
	}
	return true
}

func emitNearDupFindings(path string, cluster []nearDupMember) []scan.Finding {
	sort.Slice(cluster, func(i, j int) bool {
		if cluster[i].t.Lineno != cluster[j].t.Lineno {
			return cluster[i].t.Lineno < cluster[j].t.Lineno
		}
		return cluster[i].qual < cluster[j].qual
	})
	first := cluster[0]
	if len(cluster) >= 3 {
		twins := make([]string, 0, len(cluster)-1)
		for _, m := range cluster[1:] {
			twins = append(twins, fmt.Sprintf("%s:%d", m.qual, m.t.Lineno))
		}
		sketch := parametrizeSketch(cluster)
		msg := fmt.Sprintf(
			"%d near-duplicate tests (%s and %s); consider consolidating:\n%s",
			len(cluster), first.qual, strings.Join(twins, ", "), sketch,
		)
		second := cluster[1]
		return []scan.Finding{{
			File:            path,
			Line:            first.t.Lineno,
			Rule:            "near-duplicate-test",
			Severity:        "note",
			Message:         msg,
			QualName:        first.qual,
			RelatedLine:     second.t.Lineno,
			RelatedQualName: second.qual,
		}}
	}
	later := cluster[1]
	return []scan.Finding{{
		File:            path,
		Line:            later.t.Lineno,
		Rule:            "near-duplicate-test",
		Severity:        "note",
		Message:         fmt.Sprintf("test body is nearly identical to %s:%d", first.qual, first.t.Lineno),
		QualName:        later.qual,
		RelatedLine:     first.t.Lineno,
		RelatedQualName: first.qual,
	}}
}

func NewNearDuplicateTest() scan.Rule {
	return nearDuplicateTest{}
}

var (
	nearDupPosName = regexp.MustCompile(`(?i)(?:^|_)(accept|accepts|passes|valid|success|succeeds|allowed|permits|happy)(?:_|$)|ok_|_ok$`)
	nearDupNegName = regexp.MustCompile(`(?i)(?:^|_)(reject|rejects|fails|invalid|error|raises|raise|forbidden|blocked|denied|missing|unauthorized|not_found|bad_request)(?:_|$)`)
	nearDupHTTPRe  = regexp.MustCompile(`\b([1-5][0-9]{2})\b`)
	nearDupStrLit  = regexp.MustCompile(`(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')`)
	nearDupNumLit  = regexp.MustCompile(`\b\d+\b`)
)

func nearDupPolarity(t parse.TestFunc) string {
	name := t.Name
	hasPos := nearDupPosName.MatchString(name)
	hasNeg := nearDupNegName.MatchString(name)
	if hasPos && !hasNeg {
		return "pos"
	}
	if hasNeg && !hasPos {
		return "neg"
	}

	if t.HasRaises || len(t.Raises) > 0 {
		return "neg"
	}
	for _, c := range t.Calls {
		cl := strings.ToLower(c.Name)
		if strings.Contains(cl, "raises") || strings.Contains(cl, "assertraises") {
			return "neg"
		}
	}

	httpPol := ""
	for _, a := range t.Asserts {
		text := a.Text + " " + a.Right
		for _, m := range nearDupHTTPRe.FindAllString(text, -1) {
			code, err := strconv.Atoi(m)
			if err != nil {
				continue
			}
			p := httpPolarity(code)
			if p == "" {
				continue
			}
			if httpPol == "" {
				httpPol = p
			} else if httpPol != p {
				return "" // mixed in one test
			}
		}
		tl := strings.ToLower(a.Text)
		if strings.HasPrefix(tl, "not ") || strings.Contains(tl, " is not ") ||
			strings.Contains(tl, " is false") || strings.Contains(tl, "== false") ||
			strings.Contains(tl, "!= true") {
			if httpPol == "" {
				return "neg"
			}
		}
	}
	return httpPol
}

func httpPolarity(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "pos"
	case code >= 400 && code < 600:
		return "neg"
	default:
		return ""
	}
}

// significantAssertLiterals collects enum-like / HTTP status expectations from asserts.
// Ordinary data strings (e.g. "copy_of_a") are ignored so parametrize-worthy
// clusters still form after BodyNorm collapses literals.
func significantAssertLiterals(t parse.TestFunc) string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, a := range t.Asserts {
		for _, lit := range nearDupStrLit.FindAllString(a.Text+" "+a.Right, -1) {
			inner := strings.Trim(lit, `"'`)
			if isEnumLikeString(inner) {
				add("s:" + inner)
			}
		}
		for _, m := range nearDupNumLit.FindAllString(a.Right, -1) {
			n, err := strconv.Atoi(m)
			if err != nil {
				continue
			}
			if n >= 100 && n <= 599 {
				add("n:" + m)
			}
		}
		right := strings.TrimSpace(a.Right)
		if right != "" && !strings.ContainsAny(right, `'"`) {
			leaf := leafName(right)
			if isEnumLikeIdent(leaf) {
				add("e:" + leaf)
			}
		}
	}
	sort.Strings(out)
	return strings.Join(out, "|")
}

func isEnumLikeString(s string) bool {
	if s == "" || strings.Contains(s, " ") {
		return false
	}
	// MANAGED_IDENTITY / DEFAULT_CHAIN style.
	hasLetter := false
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			hasLetter = true
		case unicode.IsDigit(r) || r == '_':
			// ok
		default:
			return false
		}
	}
	return hasLetter
}

func isEnumLikeIdent(s string) bool {
	if s == "" || strings.Contains(s, "(") {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if unicode.IsLower(r) {
				return false
			}
		} else if r != '_' && !unicode.IsDigit(r) {
			return false
		}
	}
	return hasLetter
}

func parametrizeSketch(cluster []nearDupMember) string {
	names := make([]string, len(cluster))
	for i, m := range cluster {
		names[i] = m.t.Name
	}
	common := commonTestNamePrefix(names)
	ids := make([]string, len(cluster))
	for i, n := range names {
		id := strings.TrimPrefix(n, common)
		id = strings.Trim(id, "_")
		if id == "" {
			id = n
		}
		ids[i] = id
	}

	rights := make([][]string, len(cluster))
	maxAsserts := 0
	for i, m := range cluster {
		for _, a := range m.t.Asserts {
			r := strings.TrimSpace(a.Right)
			if r == "" {
				r = strings.TrimSpace(a.Text)
			}
			rights[i] = append(rights[i], r)
		}
		if len(rights[i]) > maxAsserts {
			maxAsserts = len(rights[i])
		}
	}

	varyIdx := []int{}
	for ai := 0; ai < maxAsserts; ai++ {
		base := ""
		if ai < len(rights[0]) {
			base = rights[0][ai]
		}
		for j := 1; j < len(cluster); j++ {
			var b string
			if ai < len(rights[j]) {
				b = rights[j][ai]
			}
			if b != base {
				varyIdx = append(varyIdx, ai)
				break
			}
		}
	}

	cols := []string{"case_id"}
	for _, ai := range varyIdx {
		cols = append(cols, fmt.Sprintf("expected_%d", ai))
	}
	rowStrs := make([]string, 0, len(cluster))
	for i := range cluster {
		cells := []string{fmt.Sprintf("%q", ids[i])}
		for _, ai := range varyIdx {
			var val string
			if ai < len(rights[i]) {
				val = rights[i][ai]
			}
			switch {
			case val == "":
				cells = append(cells, "None")
			case looksLikePythonLiteral(val):
				cells = append(cells, val)
			default:
				cells = append(cells, fmt.Sprintf("%q", val))
			}
		}
		rowStrs = append(rowStrs, "("+strings.Join(cells, ", ")+")")
	}

	return fmt.Sprintf(
		"@pytest.mark.parametrize(%q, [\n    %s,\n])",
		strings.Join(cols, ","),
		strings.Join(rowStrs, ",\n    "),
	)
}

func commonTestNamePrefix(names []string) string {
	if len(names) == 0 {
		return ""
	}
	prefix := names[0]
	for _, n := range names[1:] {
		for len(prefix) > 0 && !strings.HasPrefix(n, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	// Prefer cutting at underscore boundaries.
	if i := strings.LastIndex(prefix, "_"); i > 0 {
		return prefix[:i+1]
	}
	return prefix
}

func looksLikePythonLiteral(s string) bool {
	if s == "" {
		return false
	}
	if (strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) ||
		(strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`)) {
		return true
	}
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	switch s {
	case "True", "False", "None":
		return true
	}
	// Attr enum-like: Mode.FOO
	if strings.Contains(s, ".") && !strings.Contains(s, "(") {
		return true
	}
	return false
}
