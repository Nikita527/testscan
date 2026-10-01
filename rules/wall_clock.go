package rules

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

// wall-clock-in-test.
//
// Naive wall-clock reads (datetime.now() / datetime.today() / datetime.utcnow()
// / date.today() without a tz argument) are reported in every project, but the
// severity depends on what the project does:
//
//   - project is timezone-aware (see detectProjectTZ): `warning`, and only when
//     the value flows into a model field / call argument / assert (facts come
//     from the Python helper: Call.Flow). Values passed into an error-path call
//     (pytest.raises body, negative-path test) or never used are not reported.
//   - otherwise: `note` (previous behaviour).
//
// tz-aware calls (`datetime.now(timezone.utc)`, `datetime.now(tz=...)`,
// `.astimezone()`, `.replace(tzinfo=...)`) are never reported.

type wallClockInTest struct{}

func (wallClockInTest) ID() string { return "wall-clock-in-test" }

func (wallClockInTest) NeedsAST() bool { return true }

func (wallClockInTest) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	fileFrozen := modelHasTimeFreeze(model)
	var tz tzInfo
	tzLoaded := false
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		if fileFrozen || testHasTimeFreeze(t) {
			continue
		}
		for _, c := range t.Calls {
			if !isWallClockCall(c.Name) || c.Argc > 0 || c.AwareChain {
				continue
			}
			if !tzLoaded {
				tz = projectTZ(file.ProjectRoot)
				tzLoaded = true
			}
			if !tz.Aware {
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     c.Lineno,
					Rule:     "wall-clock-in-test",
					Severity: "note",
					Message:  "wall-clock read (datetime.now / date.today) without freeze; prefer freezegun or time-machine",
					Fix:      "freeze time with `@freeze_time(...)` (freezegun / time-machine) instead of calling `" + wallClockCallLabel(c.Name) + "`",
					QualName: q,
				})
				continue
			}
			if !wallClockFlowMatters(t, c) {
				continue
			}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     c.Lineno,
				Rule:     "wall-clock-in-test",
				Severity: "warning",
				Message:  tz.message(c.Name),
				Fix:      wallClockAwareFix(file.Content, c),
				QualName: q,
			})
		}
	}
	return findings
}

// wallClockFlowMatters: the naive value reaches an assert, a model/factory
// call, or (outside negative-path tests) another call's argument.
func wallClockFlowMatters(t parse.TestFunc, c parse.Call) bool {
	if c.InRaises {
		return false
	}
	callArg := false
	for _, f := range c.Flow {
		switch f {
		case "assert", "model_arg":
			return true
		case "call_arg":
			callArg = true
		}
	}
	return callArg && !testHasNegativePath(t, defaultNegativeNames)
}

// wallClockCallLabel is the short call as written, e.g. `datetime.now()`.
func wallClockCallLabel(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) > 2 {
		name = strings.Join(parts[len(parts)-2:], ".")
	}
	return name + "()"
}

// wallClockAwareFix names the replacement; `datetime.now().date()` on the
// finding's line maps to `timezone.localdate()` like `date.today()`.
func wallClockAwareFix(content []byte, c parse.Call) string {
	label := wallClockCallLabel(c.Name)
	repl := wallClockReplacement(c.Name)
	if lineHasDateChain(content, c.Lineno, label) {
		label = strings.TrimSuffix(label, "()") + "().date()"
		repl = "timezone.localdate()"
	}
	return "replace `" + label + "` with `" + repl + "`, or freeze time"
}

// lineHasDateChain reports whether `<call>().date()` appears on line lineno.
func lineHasDateChain(content []byte, lineno int, label string) bool {
	if lineno < 1 {
		return false
	}
	lines := strings.Split(string(content), "\n")
	if lineno > len(lines) {
		return false
	}
	short := strings.TrimSuffix(label, "()")
	return strings.Contains(strings.ReplaceAll(lines[lineno-1], " ", ""), short+"().date()")
}

// wallClockReplacement names the timezone-aware Django-style replacement.
func wallClockReplacement(name string) string {
	n := strings.ToLower(name)
	if strings.HasSuffix(n, "date.today") && !strings.HasSuffix(n, "datetime.today") {
		return "timezone.localdate()"
	}
	return "timezone.now()"
}

func isWallClockCall(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "datetime.now", "datetime.utcnow", "datetime.today",
		"datetime.datetime.now", "datetime.datetime.utcnow", "datetime.datetime.today",
		"date.today", "datetime.date.today":
		return true
	}
	leaf := strings.ToLower(leafName(name))
	if leaf == "now" || leaf == "utcnow" || leaf == "today" {
		base := strings.ToLower(name)
		return strings.Contains(base, "datetime") || strings.Contains(base, "date")
	}
	return false
}

func modelHasTimeFreeze(model parse.Model) bool {
	for _, imp := range model.Imports {
		blob := strings.ToLower(imp.Module + " " + strings.Join(imp.Names, " "))
		if strings.Contains(blob, "freezegun") ||
			strings.Contains(blob, "time_machine") ||
			strings.Contains(blob, "freeze_time") {
			return true
		}
	}
	return false
}

func testHasTimeFreeze(t parse.TestFunc) bool {
	for _, d := range t.Decorators {
		dl := strings.ToLower(d)
		if strings.Contains(dl, "freeze_time") ||
			strings.Contains(dl, "freezegun") ||
			strings.Contains(dl, "time_machine") ||
			strings.Contains(dl, "travel(") {
			return true
		}
	}
	for _, c := range t.Calls {
		cl := strings.ToLower(c.Name)
		leaf := strings.ToLower(leafName(c.Name))
		if strings.Contains(cl, "freeze_time") ||
			strings.Contains(cl, "time_machine") ||
			strings.HasSuffix(cl, ".travel") ||
			leaf == "travel" {
			return true
		}
	}
	for _, f := range t.Fixtures {
		fl := strings.ToLower(f)
		if strings.Contains(fl, "freezer") || fl == "freezer" {
			return true
		}
	}
	return false
}

func NewWallClockInTest() scan.Rule {
	return wallClockInTest{}
}

// ---------------------------------------------------------------------------
// Project timezone awareness
// ---------------------------------------------------------------------------

// tzInfo describes whether the project works with timezone-aware dates.
type tzInfo struct {
	Aware bool
	// API is the detected project API: "localdate", "now", or "" (settings only).
	API string
}

func (i tzInfo) message(callName string) string {
	call := callName
	parts := strings.Split(callName, ".")
	if len(parts) > 2 {
		call = strings.Join(parts[len(parts)-2:], ".")
	}
	call += "()"
	switch i.API {
	case "localdate":
		return "project uses timezone-aware dates (`timezone.localdate()`); naive `" + call +
			"` diverges near midnight in non-UTC local time — use `timezone.localdate()` or freeze time"
	case "now":
		return "project uses timezone-aware dates (`timezone.now()`); naive `" + call +
			"` diverges near midnight in non-UTC local time — use `timezone.now()` / `timezone.localdate()` or freeze time"
	default:
		return "project uses timezone-aware dates (USE_TZ = True); naive `" + call +
			"` diverges near midnight in non-UTC local time — use `timezone.localdate()` or freeze time"
	}
}

const tzMaxFiles = 5000

// tzMaxVisited caps the total directory entries (files and directories) the
// project timezone walk may visit.
const tzMaxVisited = 20000

// reTZFromImport matches `from django.utils.timezone import a, b as c` including
// the parenthesised multi-line form.
var reTZFromImport = regexp.MustCompile(`(?m)^[ \t]*from[ \t]+django\.utils\.timezone[ \t]+import[ \t]+(?:\(([^)]*)\)|([^\n]*))`)

var (
	tzSkipDirs = map[string]bool{
		".venv": true, "venv": true, "node_modules": true, ".git": true, "site-packages": true,
		"tests": true, "test": true, "migrations": true, "__pycache__": true, "htmlcov": true,
		"build": true, "dist": true, ".tox": true, "testdata": true,
	}
	reUseTZ      = regexp.MustCompile(`(?m)^[ \t]*USE_TZ[ \t]*=[ \t]*True\b`)
	reDjangoSets = regexp.MustCompile(`DJANGO_SETTINGS_MODULE["']?\s*[=,:]\s*["']?([\w.]+)`)

	tzMu    sync.Mutex
	tzCache = map[string]*tzEntry{}
)

type tzEntry struct {
	once sync.Once
	info tzInfo
}

// projectTZ detects (once per root per process) whether the project is
// timezone-aware: Django settings with USE_TZ = True (module from
// DJANGO_SETTINGS_MODULE in pytest config / manage.py, else any settings*.py or
// settings/ package file), or non-test sources using django.utils.timezone
// now/localdate. The source walk is bounded (tzSkipDirs, tzMaxFiles).
func projectTZ(root string) tzInfo {
	if root == "" {
		root, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	tzMu.Lock()
	e := tzCache[root]
	if e == nil {
		e = &tzEntry{}
		tzCache[root] = e
	}
	tzMu.Unlock()
	e.once.Do(func() { e.info = detectProjectTZ(root) })
	return e.info
}

func detectProjectTZ(root string) tzInfo {
	settingsAware := settingsModuleUsesTZ(root)
	usesLocaldate, usesNow := false, false

	count, visited := 0, 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		visited++
		if visited > tzMaxVisited {
			return filepath.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (tzSkipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".py") || name == "conftest.py" ||
			strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.py") {
			return nil
		}
		count++
		if count > tzMaxFiles {
			return filepath.SkipAll
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.HasPrefix(name, "settings") || strings.EqualFold(filepath.Base(filepath.Dir(path)), "settings") {
			if reUseTZ.Match(data) {
				settingsAware = true
			}
		}
		if !usesLocaldate || !usesNow {
			l, n := scanTimezoneUse(data)
			usesLocaldate = usesLocaldate || l
			usesNow = usesNow || n
		}
		// Both facts resolved: the answer cannot change any more.
		if settingsAware && usesLocaldate {
			return filepath.SkipAll
		}
		return nil
	})

	info := tzInfo{Aware: settingsAware || usesLocaldate || usesNow}
	switch {
	case usesLocaldate:
		info.API = "localdate"
	case usesNow:
		info.API = "now"
	}
	return info
}

// scanTimezoneUse reports django.utils.timezone localdate()/now() usage. For
// `from django.utils.timezone import ...` the flags follow the imported names.
func scanTimezoneUse(data []byte) (localdate, now bool) {
	if bytes.Contains(data, []byte("timezone.localdate(")) {
		localdate = true
	}
	if bytes.Contains(data, []byte("timezone.now(")) {
		now = true
	}
	for _, m := range reTZFromImport.FindAllSubmatch(data, -1) {
		list := string(m[1])
		if list == "" {
			list = string(m[2])
		}
		if i := strings.Index(list, "#"); i >= 0 {
			list = list[:i]
		}
		for _, part := range strings.Split(list, ",") {
			fields := strings.Fields(part)
			if len(fields) == 0 {
				continue
			}
			switch fields[0] {
			case "localdate":
				localdate = true
			case "now":
				now = true
			}
		}
	}
	return localdate, now
}

// settingsModuleUsesTZ resolves DJANGO_SETTINGS_MODULE from pytest config files
// (and manage.py) and checks USE_TZ = True in that module (or, for a settings
// package, in its sibling modules, since `test.py` usually does `from .base import *`).
func settingsModuleUsesTZ(root string) bool {
	for _, cfg := range []string{"pyproject.toml", "pytest.ini", "setup.cfg", "tox.ini", "manage.py"} {
		data, err := os.ReadFile(filepath.Join(root, cfg))
		if err != nil {
			continue
		}
		m := reDjangoSets.FindSubmatch(data)
		if m == nil {
			continue
		}
		rel := filepath.FromSlash(strings.ReplaceAll(string(m[1]), ".", "/"))
		for _, base := range []string{root, filepath.Join(root, "src")} {
			var files []string
			if st, err := os.Stat(filepath.Join(base, rel+".py")); err == nil && !st.IsDir() {
				files = append(files, filepath.Join(base, rel+".py"))
				dir := filepath.Dir(filepath.Join(base, rel+".py"))
				if entries, err := os.ReadDir(dir); err == nil && strings.Contains(strings.ToLower(filepath.Base(dir)), "settings") {
					for _, e := range entries {
						if !e.IsDir() && strings.HasSuffix(e.Name(), ".py") {
							files = append(files, filepath.Join(dir, e.Name()))
						}
					}
				}
			} else if st, err := os.Stat(filepath.Join(base, rel, "__init__.py")); err == nil && !st.IsDir() {
				files = append(files, filepath.Join(base, rel, "__init__.py"))
			}
			for _, f := range files {
				if b, err := os.ReadFile(f); err == nil && reUseTZ.Match(b) {
					return true
				}
			}
		}
	}
	return false
}
