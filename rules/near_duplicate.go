package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type nearDuplicateTest struct{}

func (nearDuplicateTest) ID() string { return "near-duplicate-test" }

func (nearDuplicateTest) NeedsAST() bool { return true }

// nearDupMaxBetween is the number of other tests allowed between two twins
// (in the same scope) for them to still be considered adjacent.
const nearDupMaxBetween = 1

// nearDupMaxSpan caps the size (source lines) of tests considered for
// parametrize: large bodies with big structured inputs (dict/list payloads)
// are separate scenarios whose differences a parametrize table would obscure.
const nearDupMaxSpan = 15

type nearDupMember struct {
	t          parse.TestFunc
	qual       string
	polarity   string // "pos" | "neg" | ""
	sut        string
	assertSig  string // assertions incl. RHS literals, raises/match args
	doc        string // normalized docstring ("" if none)
	decorators string
	pos        int // index among tests of the same scope (class)
}

func (nearDuplicateTest) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(file.Content), "\n")

	// Position of each test among the tests of its own scope.
	scopeCount := map[string]int{}
	posOf := make([]int, len(model.Tests))
	order := make([]int, len(model.Tests))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return model.Tests[order[a]].Lineno < model.Tests[order[b]].Lineno
	})
	for _, idx := range order {
		sc := model.Tests[idx].ClassName
		posOf[idx] = scopeCount[sc]
		scopeCount[sc]++
	}

	byNorm := map[string][]nearDupMember{}
	for i, t := range model.Tests {
		norm := t.BodyNorm
		if norm == "" || t.IsEmpty {
			continue
		}
		doc, assertLines := nearDupSourceInfo(lines, t)
		byNorm[norm] = append(byNorm[norm], nearDupMember{
			t:          t,
			qual:       qualName(t),
			polarity:   nearDupPolarity(t),
			sut:        primarySUTCall(t),
			assertSig:  nearDupAssertSig(t, assertLines),
			doc:        doc,
			decorators: strings.Join(t.Decorators, " | "),
			pos:        posOf[i],
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
	// Compatibility is not transitive (empty docstring / polarity / SUT match
	// anything), so a candidate joins a cluster only if it is compatible with
	// EVERY member already in it. Deterministic order: by line number.
	sorted := append([]nearDupMember(nil), group...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].t.Lineno < sorted[j].t.Lineno
	})
	var out [][]nearDupMember
	for _, m := range sorted {
		placed := false
		for ci, c := range out {
			ok := true
			for _, o := range c {
				if !nearDupCompatible(o, m) {
					ok = false
					break
				}
			}
			if ok {
				out[ci] = append(out[ci], m)
				placed = true
				break
			}
		}
		if !placed {
			out = append(out, []nearDupMember{m})
		}
	}
	return out
}

// nearDupCompatible reports whether two tests with the same normalized body are
// a parametrize candidate: adjacent, same decorators, same assertions
// (including expected values), non-antonymous names, and not two distinct
// documented scenarios.
func nearDupCompatible(a, b nearDupMember) bool {
	if a.qual == b.qual {
		return false
	}
	if nearDupSpan(a.t) > nearDupMaxSpan || nearDupSpan(b.t) > nearDupMaxSpan {
		return false
	}
	if a.t.ClassName != b.t.ClassName {
		return false
	}
	d := a.pos - b.pos
	if d < 0 {
		d = -d
	}
	if d-1 > nearDupMaxBetween {
		return false
	}
	if a.decorators != b.decorators {
		return false
	}
	if a.doc != "" && b.doc != "" && a.doc != b.doc {
		return false
	}
	if nearDupAntonymNames(a.t.Name, b.t.Name) {
		return false
	}
	// Opposite polarity (accept <-> reject, raises <-> no, 2xx <-> 4xx).
	if a.polarity != "" && b.polarity != "" && a.polarity != b.polarity {
		return false
	}
	// Different SUT calls.
	if a.sut != "" && b.sut != "" && a.sut != b.sut {
		return false
	}
	// Expected values / raises / match args must be identical: only inputs may differ.
	return a.assertSig == b.assertSig
}

func nearDupSpan(t parse.TestFunc) int {
	if t.EndLineno < t.Lineno {
		return 0
	}
	return t.EndLineno - t.Lineno + 1
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
		msg := fmt.Sprintf(
			"%d tests (%s and %s) have the same body and assertions, differing only in input literals — candidate for @pytest.mark.parametrize",
			len(cluster), first.qual, strings.Join(twins, ", "),
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
			Fix: fmt.Sprintf("merge %s and %s into one test with `@pytest.mark.parametrize` over the differing inputs",
				first.qual, strings.Join(twinNames(cluster[1:]), ", ")),
		}}
	}
	later := cluster[1]
	return []scan.Finding{{
		File:     path,
		Line:     later.t.Lineno,
		Rule:     "near-duplicate-test",
		Severity: "note",
		Message: fmt.Sprintf(
			"same body and assertions as %s:%d, differing only in input literals — candidate for @pytest.mark.parametrize",
			first.qual, first.t.Lineno,
		),
		QualName:        later.qual,
		RelatedLine:     first.t.Lineno,
		RelatedQualName: first.qual,
		Fix: fmt.Sprintf("merge %s into %s with `@pytest.mark.parametrize` over the differing inputs",
			later.qual, first.qual),
	}}
}

func twinNames(ms []nearDupMember) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.qual)
	}
	return out
}

func NewNearDuplicateTest() scan.Rule {
	return nearDuplicateTest{}
}

var (
	nearDupPosName = regexp.MustCompile(`(?i)(?:^|_)(accept|accepts|passes|valid|success|succeeds|allowed|permits|happy)(?:_|$)|ok_|_ok$`)
	nearDupNegName = regexp.MustCompile(`(?i)(?:^|_)(reject|rejects|fails|invalid|error|raises|raise|forbidden|blocked|denied|missing|unauthorized|not_found|bad_request)(?:_|$)`)
	nearDupHTTPRe  = regexp.MustCompile(`\b([1-5][0-9]{2})\b`)
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

// nearDupAssertRe matches statements that carry the test's expectations:
// assert statements, assert-style helpers/methods and pytest.raises/assertRaises.
var nearDupAssertRe = regexp.MustCompile(`^assert\b|\bassert\w*\(|\braises\(`)

// nearDupAssertSig builds a canonical signature of everything a test expects:
// the parsed assertions (full text incl. right-hand sides) plus the source of
// assertion-like statements (pytest.raises(E, match=...), helper asserts).
func nearDupAssertSig(t parse.TestFunc, assertLines []string) string {
	parts := make([]string, 0, len(t.Asserts)+len(assertLines))
	for _, a := range t.Asserts {
		parts = append(parts, strings.Join(strings.Fields(a.Text+" => "+a.Right), " "))
	}
	parts = append(parts, assertLines...)
	return strings.Join(parts, "\n")
}

// nearDupSourceInfo extracts the docstring and assertion-like statements of a
// test from the file source. Missing/invalid line ranges yield empty results.
func nearDupSourceInfo(lines []string, t parse.TestFunc) (doc string, assertLines []string) {
	start := t.Lineno - 1
	end := t.EndLineno
	if start < 0 || start >= len(lines) {
		return "", nil
	}
	if end < t.Lineno {
		end = t.Lineno
	}
	if end > len(lines) {
		end = len(lines)
	}
	seg := lines[start:end]

	// Skip the signature (may span several lines).
	depth := 0
	bodyStart := len(seg)
	for i, ln := range seg {
		depth += nearDupBracketDelta(ln)
		if depth <= 0 && strings.HasSuffix(nearDupStripComment(ln), ":") {
			bodyStart = i + 1
			break
		}
	}
	body := seg[bodyStart:]

	// Docstring.
	i := 0
	for i < len(body) && strings.TrimSpace(body[i]) == "" {
		i++
	}
	if i < len(body) {
		doc, i = nearDupDocstring(body, i)
	}

	// Assertion-like statements (with continuation lines).
	for i < len(body) {
		tl := strings.TrimSpace(body[i])
		if tl == "" || !nearDupAssertRe.MatchString(tl) {
			i++
			continue
		}
		stmt := []string{tl}
		d := nearDupBracketDelta(tl)
		for (d > 0 || strings.HasSuffix(strings.TrimSpace(stmt[len(stmt)-1]), `\`)) && i+1 < len(body) {
			i++
			nl := strings.TrimSpace(body[i])
			stmt = append(stmt, nl)
			d += nearDupBracketDelta(nl)
		}
		assertLines = append(assertLines, strings.Join(strings.Fields(strings.Join(stmt, " ")), " "))
		i++
	}
	return doc, assertLines
}

// nearDupDocstring returns the normalized docstring starting at body[i] (if
// any) and the index of the first line after it.
func nearDupDocstring(body []string, i int) (string, int) {
	tl := strings.TrimSpace(body[i])
	lower := strings.ToLower(tl)
	prefixLen := 0
	for prefixLen < len(lower) && prefixLen < 2 && strings.ContainsRune("rub", rune(lower[prefixLen])) {
		prefixLen++
	}
	rest := tl[prefixLen:]
	for _, q := range []string{`"""`, `'''`} {
		if strings.HasPrefix(rest, q) {
			text := rest[3:]
			j := i
			for {
				if k := strings.Index(text, q); k >= 0 {
					return strings.Join(strings.Fields(text[:k]), " "), j + 1
				}
				j++
				if j >= len(body) {
					return strings.Join(strings.Fields(text), " "), j
				}
				text += " " + body[j]
			}
		}
	}
	for _, q := range []string{`"`, `'`} {
		if len(rest) >= 2 && strings.HasPrefix(rest, q) && strings.HasSuffix(rest, q) &&
			!strings.Contains(rest[1:len(rest)-1], q) {
			return strings.Join(strings.Fields(rest[1:len(rest)-1]), " "), i + 1
		}
	}
	return "", i
}

func nearDupBracketDelta(s string) int {
	d := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#':
			return d
		case c == '(' || c == '[' || c == '{':
			d++
		case c == ')' || c == ']' || c == '}':
			d--
		}
	}
	return d
}

func nearDupStripComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#':
			return strings.TrimSpace(s[:i])
		}
	}
	return strings.TrimSpace(s)
}

// Antonym token pairs: tests whose names differ by one of these describe
// opposite scenarios, not parametrize rows.
var nearDupAntonyms = [][2]string{
	{"passes", "dropped"}, {"pass", "drop"}, {"passes", "drops"},
	{"demotes", "retries"}, {"demote", "retry"}, {"promotes", "demotes"},
	{"accept", "reject"}, {"accepts", "rejects"}, {"accepted", "rejected"},
	{"allow", "deny"}, {"allows", "denies"}, {"allowed", "denied"}, {"allowed", "forbidden"}, {"allowed", "blocked"},
	{"valid", "invalid"}, {"enabled", "disabled"}, {"enable", "disable"}, {"enables", "disables"},
	{"with", "without"}, {"succeeds", "fails"}, {"success", "failure"}, {"success", "fail"}, {"succeed", "fail"},
	{"ok", "error"}, {"present", "missing"}, {"present", "absent"}, {"found", "missing"},
	{"include", "exclude"}, {"includes", "excludes"}, {"included", "excluded"},
	{"add", "remove"}, {"adds", "removes"}, {"added", "removed"},
	{"create", "delete"}, {"creates", "deletes"}, {"created", "deleted"},
	{"true", "false"}, {"on", "off"}, {"open", "close"}, {"opens", "closes"},
	{"positive", "negative"}, {"min", "max"}, {"first", "last"}, {"upper", "lower"},
	{"start", "stop"}, {"starts", "stops"}, {"raises", "returns"}, {"required", "optional"},
	{"empty", "nonempty"}, {"before", "after"}, {"increase", "decrease"}, {"increases", "decreases"},
	{"keeps", "drops"}, {"keeps", "removes"}, {"inside", "outside"}, {"match", "mismatch"},
	{"matches", "mismatches"}, {"hit", "miss"}, {"hits", "misses"}, {"active", "inactive"},
}

var nearDupNegTokens = map[string]bool{
	"not": true, "no": true, "without": true, "never": true, "non": true, "nor": true,
}

func nearDupAntonymNames(a, b string) bool {
	ta := nearDupTokens(a)
	tb := nearDupTokens(b)
	// "not_x"/"no_x"/"without_x" on one side only.
	negA, negB := false, false
	for t := range ta {
		if nearDupNegTokens[t] {
			negA = true
		}
	}
	for t := range tb {
		if nearDupNegTokens[t] {
			negB = true
		}
	}
	if negA != negB {
		return true
	}
	for _, p := range nearDupAntonyms {
		if (ta[p[0]] && tb[p[1]]) || (ta[p[1]] && tb[p[0]]) {
			return true
		}
	}
	// Generic negating prefixes: valid/invalid, able/unable, ...
	for x := range ta {
		for y := range tb {
			if nearDupNegPrefixed(x, y) || nearDupNegPrefixed(y, x) {
				return true
			}
		}
	}
	return false
}

func nearDupNegPrefixed(base, neg string) bool {
	for _, p := range []string{"in", "un", "dis", "non", "im", "mis"} {
		if strings.HasPrefix(neg, p) && neg[len(p):] == base && len(base) >= 4 {
			return true
		}
	}
	return false
}

func nearDupTokens(name string) map[string]bool {
	out := map[string]bool{}
	for _, t := range strings.Split(strings.ToLower(name), "_") {
		if t != "" && t != "test" {
			out[t] = true
		}
	}
	return out
}
