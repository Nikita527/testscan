package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

// ---------------------------------------------------------------------------
// Data-flow: "does any check in this test depend on the SUT result?"
//
// The Python helper only exports facts (parse.Flow, Assert.Reads/Calls/MState,
// Call.Reads, nested-callback writes); SUT classification stays in Go
// (SUTContext.IsDataflowSUT), so there is exactly one SUT definition.
//
// A name is "tainted since line L" when its value or state may come from the
// SUT. Sources:
//
//   - a SUT call (project call, test-client request, helper handed the client):
//     every name assigned from the statement containing it, every name read by that
//     statement (arguments, including fakes nested in other calls: side effects on
//     fixtures/objects the SUT mutated), and, transitively, the names those
//     arguments were built from (`svc = Fake(state); sut(svc)` taints `state`);
//   - a state read after the first SUT call: `x.refresh_from_db()`, `Model.objects.*`,
//     calls on fixtures or on test-local helpers (`rows = run_fanout()`), plus
//     output-observing fixtures (caplog, capsys, mailoutbox, tmp_path...);
//   - `with pytest.raises(...) as exc` / `with CaptureQueriesContext(c) as q` /
//     `except E as exc` whose block runs a SUT call, state read or tainted call;
//   - names mutated by callbacks (nested defs / lambdas) once the SUT has run;
//   - the recorded state (`m.call_args`, `m.called`) of a mock that is wired
//     into the test's activity.
//
// Taint propagates through assignment, augmented assignment, tuple unpacking,
// attribute/subscript stores (to the root; `self.x` is tracked as its own path),
// for-targets, with-as, comprehensions, f-strings (all name reads of the value
// expression flow to the target) and calls (a call taking a tainted argument or
// called on a tainted receiver yields a tainted result).
//
// A check depends on the SUT when it reads a tainted name (at or after the
// taint line), contains a SUT call / state read, reads a project symbol
// (constants and classes imported from the project), is a pytest.raises /
// query-count block around such a call, is an assert helper call (assert_*,
// check_*, expect_*, self.assert*, configured assert-helpers, local helpers that
// assert) fed with tainted data or callbacks, or is a mock assertion on a mock
// wired into what the test executes (see wired below).
// ---------------------------------------------------------------------------

var mockCtorLeaves = map[string]bool{
	"MagicMock": true, "Mock": true, "AsyncMock": true, "PropertyMock": true,
	"NonCallableMock": true, "NonCallableMagicMock": true, "create_autospec": true,
}

// observedNames are fixtures/globals that expose what code under test produced.
var observedNames = map[string]bool{
	"caplog": true, "capsys": true, "capfd": true, "capsysbinary": true, "capfdbinary": true,
	"mailoutbox": true, "mail": true, "outbox": true,
	"tmp_path": true, "tmpdir": true, "tmp_path_factory": true, "tmpdir_factory": true,
}

var wiringNeutralCalls = map[string]bool{
	"isinstance": true, "len": true, "id": true, "print": true, "type": true,
	"repr": true, "str": true, "hasattr": true, "callable": true,
}

var mockStateAttrs = []string{
	".called", ".call_count", ".call_args", ".call_args_list", ".mock_calls", ".method_calls",
}

// MockAssertion is one mock.assert_* call and whether the asserted mock is wired
// into what the test executes.
type MockAssertion struct {
	Call   int
	Lineno int
	Root   string
	Wired  bool
}

// Dataflow is the per-test result shared by no-assert, mock-only-assert and
// mock-tautology.
type Dataflow struct {
	// Test is a copy of the analyzed test with Assert.DependsOnSUT,
	// HasSUTCall and AnySUTDependentAssert filled.
	Test parse.TestFunc
	// HasSUT: the test calls the SUT (project call or test-client request).
	HasSUT bool
	// HasChecks: any assert / raises / helper / strict HTTP mock exists.
	HasChecks bool
	// DependentChecks counts checks (asserts, raises blocks, helper calls,
	// wired mock assertions) that depend on the SUT.
	DependentChecks int
	// MockAsserts lists mock.assert_* calls.
	MockAsserts []MockAssertion
	// Tautology[i] is true when assert i compares only mock-configuration data.
	Tautology []bool
}

// NoDependentCheck is the shared predicate: the test has checks and exercises the
// SUT, yet none of the checks depends on the SUT result.
func (d *Dataflow) NoDependentCheck() bool {
	return d.HasSUT && d.HasChecks && d.DependentChecks == 0
}

// AnnotateDataflow returns model with every test's DependsOnSUT / HasSUTCall /
// AnySUTDependentAssert filled (tests are copied; the input is not modified).
func AnnotateDataflow(file scan.File, model parse.Model) parse.Model {
	ctx := NewSUTContext(file, model)
	out := model
	out.Tests = make([]parse.TestFunc, len(model.Tests))
	for i, t := range model.Tests {
		out.Tests[i] = AnalyzeDataflow(file, model, ctx, t).Test
	}
	return out
}

func isMockAssertName(name string) bool {
	leaf := leafName(name)
	return strings.HasPrefix(leaf, "assert_called") || leaf == "assert_has_calls" ||
		leaf == "assert_any_call" || leaf == "assert_not_called"
}

func isRaisesCallName(name string) bool {
	switch name {
	case "pytest.raises", "pytest.warns", "raises", "warns":
		return true
	}
	return false
}

// isQueryCountCheck: django_assert_num_queries(3) / assertNumQueries(3) context managers.
func isQueryCountCheck(name string) bool {
	leaf := leafName(name)
	return strings.Contains(leaf, "assert_num_queries") || strings.Contains(leaf, "assert_max_num_queries") ||
		strings.Contains(leaf, "assertNumQueries") || strings.Contains(leaf, "assertMaxNumQueries")
}

// hasQueryCountCheck reports a query-count assertion context in the test.
func hasQueryCountCheck(t parse.TestFunc) bool {
	for _, c := range t.Calls {
		if isQueryCountCheck(c.Name) {
			return true
		}
	}
	return false
}

// pathRoot is the tracked name of a dotted expression: `m.x.y` -> `m`, `self.m.x` -> `self.m`.
func pathRoot(name string) string {
	head, rest := firstSegment(name)
	if (head == "self" || head == "cls") && rest != "" {
		r := rest[1:]
		if i := strings.Index(r, "."); i >= 0 {
			r = r[:i]
		}
		return head + "." + r
	}
	return head
}

// mockishName: a name that is a mock by construction or by convention.
func mockishName(t parse.TestFunc, localMock map[string]bool, name string) bool {
	if localMock[name] {
		return true
	}
	if origin, ok := t.VarOrigins[name]; ok && strings.Contains(strings.ToLower(origin), "patch") {
		return true
	}
	ln := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(name, "self."), "cls."))
	return ln == "m" || strings.Contains(ln, "mock") || strings.HasPrefix(ln, "m_") ||
		strings.HasSuffix(ln, "_m") || strings.Contains(ln, "stub")
}

type flowState struct {
	tainted map[string]int
}

func (s *flowState) taint(name string, line int) {
	if name == "" {
		return
	}
	if cur, ok := s.tainted[name]; !ok || line < cur {
		s.tainted[name] = line
	}
}

func (s *flowState) anyTainted(reads []string, line int) bool {
	for _, r := range reads {
		if since, ok := s.tainted[r]; ok && since <= line {
			return true
		}
	}
	return false
}

// AnalyzeDataflow computes the dependency facts of one test.
func AnalyzeDataflow(file scan.File, model parse.Model, ctx *SUTContext, t parse.TestFunc) *Dataflow {
	df := &Dataflow{}
	calls := t.Calls
	sut := make([]bool, len(calls))
	firstSUT := -1
	for i, c := range calls {
		if ctx.IsDataflowSUT(t, c) {
			sut[i] = true
			df.HasSUT = true
			if firstSUT < 0 || c.Lineno < firstSUT {
				firstSUT = c.Lineno
			}
		}
	}
	st := &flowState{tainted: map[string]int{}}
	nestedDef := map[string]bool{}
	for _, n := range t.NestedDefs {
		nestedDef[n] = true
	}

	isCheck := func(c parse.Call) bool {
		if isMockAssertName(c.Name) || isRaisesCallName(c.Name) {
			return false
		}
		return isAssertHelperCall(c.Name, file.AssertHelpers) ||
			strings.HasPrefix(leafName(c.Name), "expect_") ||
			isQueryCountCheck(c.Name) ||
			helperHasAssert(c.Name, model)
	}

	// Mocks and their wiring.
	localMock := map[string]bool{}
	for name, origin := range t.VarOrigins {
		if mockCtorLeaves[leafName(origin)] {
			localMock[name] = true
		}
	}
	hasActivity := false
	for _, c := range calls {
		if isFrameworkOrMockCall(c.Name) || c.RecvLiteral || localMock[pathRoot(c.Name)] || isCheck(c) {
			continue
		}
		hasActivity = true
		break
	}
	passed := func(root string) bool {
		for _, c := range calls {
			if isMockAssertName(c.Name) || localMock[pathRoot(c.Name)] || wiringNeutralCalls[leafName(c.Name)] {
				continue
			}
			if containsString(c.Reads, root) {
				return true
			}
		}
		for _, f := range t.Flows {
			if f.Kind == "expr" || !containsString(f.Reads, root) {
				continue
			}
			for _, tg := range f.Targets {
				if !localMock[tg] {
					return true
				}
			}
		}
		return false
	}
	wiredCache := map[string]bool{}
	wired := func(root string) bool {
		if v, ok := wiredCache[root]; ok {
			return v
		}
		v := hasActivity
		if localMock[root] {
			v = passed(root)
		}
		wiredCache[root] = v
		return v
	}
	mstateWired := func(names []string) bool {
		for _, n := range names {
			if wired(n) {
				return true
			}
		}
		return false
	}

	// Sources.
	isStateRead := func(c parse.Call) bool {
		if firstSUT < 0 || c.Lineno < firstSUT {
			return false
		}
		leaf := leafName(c.Name)
		if leaf == "refresh_from_db" {
			return true
		}
		segs := strings.Split(c.Name, ".")
		for _, seg := range segs[:len(segs)-1] {
			if seg == "objects" {
				return true
			}
		}
		root, _ := firstSegment(c.Name)
		if origin, ok := t.VarOrigins[root]; ok && strings.Contains(origin, ".objects") {
			return true
		}
		return false
	}
	// isObserver: after the SUT ran, a call on a fixture / test-local helper reads
	// the state the SUT left behind (or wraps the SUT itself).
	isObserver := func(c parse.Call) bool {
		if firstSUT < 0 || c.Lineno < firstSUT || c.RecvLiteral || isFrameworkOrMockCall(c.Name) {
			return false
		}
		root, rest := firstSegment(c.Name)
		if _, local := t.VarOrigins[root]; local {
			return false
		}
		if localMock[root] {
			return false
		}
		switch {
		case root == "self" || root == "cls":
			return strings.Count(rest, ".") >= 1
		case containsString(t.Fixtures, root):
			return true
		case nestedDef[root]:
			return true
		case ctx.IsTestSupportName(root):
			return !looksLikeClassName(leafName(c.Name))
		}
		return false
	}
	srcCall := func(ci int) bool {
		if ci < 0 || ci >= len(calls) {
			return false
		}
		return sut[ci] || isStateRead(calls[ci]) || isObserver(calls[ci])
	}
	touches := func(c parse.Call) bool {
		if isFrameworkOrMockCall(c.Name) {
			return false
		}
		if since, ok := st.tainted[pathRoot(c.Name)]; ok && since <= c.Lineno {
			return true
		}
		return st.anyTainted(c.Reads, c.Lineno)
	}
	rangeSrc := func(lo, hi int) bool {
		if hi < lo {
			hi = lo
		}
		for i, c := range calls {
			if c.Lineno < lo || c.Lineno > hi {
				continue
			}
			if srcCall(i) || touches(c) {
				return true
			}
		}
		return false
	}
	flowSrc := func(f parse.Flow) bool {
		for _, ci := range f.Calls {
			if srcCall(ci) {
				return true
			}
		}
		if (f.Kind == "with" || f.Kind == "except") && f.EndLine > 0 && rangeSrc(f.Lineno, f.EndLine) {
			return true
		}
		if st.anyTainted(f.Reads, f.Lineno) {
			return true
		}
		return len(f.MState) > 0 && mstateWired(f.MState)
	}
	flowHasSUT := func(f parse.Flow) bool {
		for _, ci := range f.Calls {
			if ci >= 0 && ci < len(calls) && sut[ci] {
				return true
			}
		}
		return false
	}

	// Seeds.
	if firstSUT >= 0 {
		for name := range observedNames {
			st.taint(name, firstSUT)
		}
		for _, w := range t.NestedWrites {
			st.taint(w, firstSUT)
		}
	}
	closed := map[string]bool{}
	closure := func(names []string, line int) {
		work := append([]string{}, names...)
		for len(work) > 0 {
			x := work[len(work)-1]
			work = work[:len(work)-1]
			if closed[x] {
				continue
			}
			closed[x] = true
			for _, g := range t.Flows {
				if g.Lineno > line || !containsString(g.Targets, x) {
					continue
				}
				for _, r := range g.Reads {
					st.taint(r, line)
					work = append(work, r)
				}
			}
		}
	}

	// Mock configuration (tautology): assigned `<mock>.return_value / side_effect`.
	configured := map[string]bool{}
	for _, a := range t.Assignments {
		if r := pathRoot(a.Target); r != "" && mockishName(t, localMock, r) {
			configured[r] = true
		}
	}
	derived := map[string]int{}
	derivedSince := func(reads []string, line int) bool {
		for _, r := range reads {
			if configured[r] {
				return true
			}
			if since, ok := derived[r]; ok && since <= line {
				return true
			}
		}
		return false
	}

	propagate := func() {
		for _, f := range t.Flows {
			if f.Top != nil && *f.Top >= 0 && *f.Top < len(calls) {
				tc := calls[*f.Top]
				if isCheck(tc) || isMockAssertName(tc.Name) {
					continue // asserting must not taint the receiver
				}
			}
			if flowSrc(f) {
				if flowHasSUT(f) {
					for _, r := range f.Reads {
						st.taint(r, f.Lineno)
					}
					closure(f.Reads, f.Lineno)
				}
				for _, tg := range f.Targets {
					st.taint(tg, f.Lineno)
				}
				continue
			}
			if len(configured) == 0 && len(derived) == 0 {
				continue
			}
			involved, foreign := false, false
			for _, ci := range f.Calls {
				if ci < 0 || ci >= len(calls) {
					continue
				}
				if configured[pathRoot(calls[ci].Name)] {
					involved = true
				} else if !isFrameworkOrMockCall(calls[ci].Name) {
					foreign = true
				}
			}
			if derivedSince(f.Reads, f.Lineno) {
				involved = true
			}
			if involved && !foreign {
				for _, tg := range f.Targets {
					if cur, ok := derived[tg]; !ok || f.Lineno < cur {
						derived[tg] = f.Lineno
					}
				}
			}
		}
	}
	propagate()
	propagate() // blocks (raises / query counters / except) depend on taint found later

	raiseTargets := map[int]bool{}
	for _, r := range t.Raises {
		raiseTargets[r.Lineno] = true
	}

	// Checks fed through assert-helper calls.
	covered := map[int]bool{}
	for _, f := range t.Flows {
		src := flowSrc(f)
		idxs := f.Calls
		if f.Top != nil && !containsInt(idxs, *f.Top) {
			idxs = append(append([]int{}, idxs...), *f.Top)
		}
		for _, ci := range idxs {
			if ci < 0 || ci >= len(calls) {
				continue
			}
			c := calls[ci]
			if !isCheck(c) {
				continue
			}
			covered[ci] = true
			df.HasChecks = true
			if checkDepends(ctx, model, t, c, src, nestedDef) {
				df.DependentChecks++
			}
		}
	}
	for ci, c := range calls {
		if covered[ci] || !isCheck(c) {
			continue
		}
		df.HasChecks = true
		if checkDepends(ctx, model, t, c, sut[ci] || st.anyTainted(c.Reads, c.Lineno), nestedDef) {
			df.DependentChecks++
		}
	}

	// Mock assertions.
	for i, c := range calls {
		if !isMockAssertName(c.Name) {
			continue
		}
		recv := strings.TrimSuffix(strings.TrimSuffix(c.Name, leafName(c.Name)), ".")
		root := pathRoot(recv)
		df.MockAsserts = append(df.MockAsserts, MockAssertion{Call: i, Lineno: c.Lineno, Root: root, Wired: wired(root)})
	}

	// Per-assert dependency.
	asserts := make([]parse.Assert, len(t.Asserts))
	copy(asserts, t.Asserts)
	df.Tautology = make([]bool, len(asserts))
	anyDep := false
	for i := range asserts {
		a := &asserts[i]
		df.HasChecks = true
		if a.Kind == "mock_method" {
			a.DependsOnSUT = mockAssertWired(df.MockAsserts, *a, wired)
		} else {
			for _, ci := range a.Calls {
				if srcCall(ci) {
					a.DependsOnSUT = true
					break
				}
			}
			if !a.DependsOnSUT {
				a.DependsOnSUT = st.anyTainted(a.Reads, a.Lineno) ||
					(len(a.MState) > 0 && mstateWired(a.MState))
			}
			if !a.DependsOnSUT {
				for _, r := range a.Reads {
					if ctx.IsProjectName(r) {
						a.DependsOnSUT = true
						break
					}
				}
			}
			if !a.DependsOnSUT {
				df.Tautology[i] = assertIsMockTautology(t, *a, configured, derivedSince)
			}
		}
		if a.DependsOnSUT {
			anyDep = true
			df.DependentChecks++
		}
	}
	// Mock assertions that exist only as calls (no Assert entry) still count.
	if len(t.Asserts) == 0 {
		for _, m := range df.MockAsserts {
			df.HasChecks = true
			if m.Wired {
				df.DependentChecks++
			}
		}
	}
	for _, r := range t.Raises {
		df.HasChecks = true
		if rangeSrc(r.Lineno, r.EndLineno) {
			df.DependentChecks++
		}
	}
	if t.NestedAssert {
		df.HasChecks = true
		df.DependentChecks++ // asserts inside callbacks: benefit of the doubt
	}
	if testHasAssertOrHelper(t, file.AssertHelpers, model) {
		df.HasChecks = true
	}
	if hasStrictHTTPMock(t) {
		df.HasChecks = true
		df.DependentChecks++
	}

	df.Test = t
	df.Test.Asserts = asserts
	df.Test.HasSUTCall = df.HasSUT
	df.Test.AnySUTDependentAssert = anyDep
	return df
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// checkDepends: assert-helper call c fed from a flow whose SUT/state source flag is src.
func checkDepends(ctx *SUTContext, model parse.Model, t parse.TestFunc, c parse.Call, src bool, nestedDef map[string]bool) bool {
	if src {
		return true
	}
	for _, r := range c.Reads {
		if nestedDef[r] {
			return true // helper(build, run): runs callbacks that exercise the SUT
		}
	}
	if len(c.Reads) == 0 && helperHasAssert(c.Name, model) {
		// local helper fed no names (assert_state()): it may read module state; benefit of the doubt
		return true
	}
	leaf := leafName(c.Name)
	if leaf == "assertRaises" || leaf == "assertRaisesRegex" || leaf == "assertWarns" {
		for _, r := range c.Reads {
			if _, ok := ctx.classify(t, parse.Call{Name: r, Lineno: c.Lineno}); ok {
				return true
			}
		}
	}
	return false
}

func mockAssertWired(ms []MockAssertion, a parse.Assert, wired func(string) bool) bool {
	for _, m := range ms {
		if m.Lineno == a.Lineno {
			return m.Wired
		}
	}
	root := a.Text
	if i := strings.Index(root, ".assert"); i >= 0 {
		root = root[:i]
	}
	return wired(pathRoot(root))
}

// assertIsMockTautology: the assert reads / calls only configured mocks (or values
// derived from them) and nothing from the SUT.
func assertIsMockTautology(t parse.TestFunc, a parse.Assert, configured map[string]bool, derivedSince func([]string, int) bool) bool {
	involved := false
	for _, ci := range a.Calls {
		if ci < 0 || ci >= len(t.Calls) {
			continue
		}
		c := t.Calls[ci]
		if configured[pathRoot(c.Name)] {
			involved = true
		} else if !isFrameworkOrMockCall(c.Name) {
			return false // a foreign call (maybe the SUT) is evaluated here
		}
	}
	for _, s := range mockStateAttrs {
		if strings.Contains(a.Text, s) {
			return false
		}
	}
	if derivedSince(a.Reads, a.Lineno) {
		involved = true
	}
	if !involved && strings.Contains(a.Text, ".return_value") && len(configured) > 0 {
		involved = true
	}
	return involved
}
