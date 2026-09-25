package scan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/Nikita527/testscan/internal/parse"
	"golang.org/x/sync/errgroup"
)

var (
	ErrRulesRequired = errors.New("rules are required")
)

type Finding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`

	// QualName is the qualified test name (Class.test_foo / test_foo).
	QualName string `json:"qual_name,omitempty"`
	// Fingerprint is a stable id (rule+file+qualname+snippet hash).
	Fingerprint string `json:"fingerprint,omitempty"`
}

// PathOverride disables rules for paths matching Path (glob).
type PathOverride struct {
	Path    string
	Disable []string
}

// File is a test source. After Walk, Run fills AST (Model*) once when any
// selected rule NeedsAST(), so multiple AST rules do not re-spawn Python.
type File struct {
	Path    string
	Content []byte

	// ModelOK means Run already invoked the parser for this file.
	ModelOK  bool
	Model    parse.Model
	ModelErr error

	// AllowHeuristicFallback permits text heuristics when AST fails.
	AllowHeuristicFallback bool

	// AssertHelpers are glob patterns for assert helper names (from config).
	AssertHelpers []string
}

type Rule interface {
	ID() string
	Check(file File) []Finding
}

// ASTRule is an optional marker: the rule needs parse.Model before Check.
type ASTRule interface {
	Rule
	NeedsAST() bool
}

type Options struct {
	Rules   []Rule
	Workers int // 0 → runtime.NumCPU()
	// Parser replaces the AST helper for Run. nil → batch pool (or parse.File).
	Parser parse.Parser
	// AllowHeuristicFallback lets rules fall back to string heuristics when AST
	// is unavailable. Default false: Run emits a parse-error note instead.
	AllowHeuristicFallback bool

	Exclude          []string
	PythonFiles      []string
	RespectGitignore bool
	PathRoot         string

	AssertHelpers   []string
	RuleSeverity    map[string]string
	Overrides       []PathOverride
	PythonFunctions []string
	PythonClasses   []string

	// CoveragePath — path to coverage.py JSON report for ProjectRule coverage mode.
	CoveragePath string
}

// ProjectInfo is passed to ProjectRule.CheckProject after per-file checks.
type ProjectInfo struct {
	PathRoot     string
	CoveragePath string
}

// ProjectRule is an optional extension for a whole-project pass (e.g. coverage).
type ProjectRule interface {
	Rule
	CheckProject(ctx context.Context, info ProjectInfo) []Finding
}

// Result is the outcome of a scan: findings plus how many test files were walked.
type Result struct {
	Findings []Finding
	Files    int
}

func Run(ctx context.Context, roots []string, opts Options) (Result, error) {
	if opts.Rules == nil {
		return Result{}, ErrRulesRequired
	}
	pathRoot := opts.PathRoot
	if pathRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			pathRoot = cwd
		}
	}

	files, err := Walk(ctx, roots, WalkOptions{
		Exclude:          opts.Exclude,
		PythonFiles:      opts.PythonFiles,
		RespectGitignore: opts.RespectGitignore,
		Root:             pathRoot,
	})
	if err != nil {
		return Result{}, err
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	needAST := anyNeedsAST(opts.Rules)

	// Always set (including empty) so a prior Run cannot leak custom discovery globs.
	if needAST {
		parse.SetDiscoveryPatterns(opts.PythonFunctions, opts.PythonClasses)
	}

	perFile := make([][]Finding, len(files))
	contents := make(map[string][]byte, len(files))
	for _, f := range files {
		contents[f.Path] = f.Content
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	var batchPool chan *parse.Batch
	if needAST && opts.Parser == nil {
		batchPool = startBatchPool(gctx, workers)
	}

	for i, file := range files {
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			f := file
			f.AllowHeuristicFallback = opts.AllowHeuristicFallback
			f.AssertHelpers = opts.AssertHelpers
			if needAST {
				f = withAST(gctx, f, opts.Parser, batchPool)
			}
			var fs []Finding
			if needAST && f.ModelOK && f.ModelErr != nil {
				fs = append(fs, parseErrorFinding(f))
			}
			for _, rule := range opts.Rules {
				if ruleDisabledForPath(rule.ID(), f.Path, pathRoot, opts.Overrides) {
					continue
				}
				fs = append(fs, rule.Check(f)...)
			}
			perFile[i] = fs
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		closeBatchPool(batchPool)
		return Result{}, err
	}
	closeBatchPool(batchPool)

	findings := make([]Finding, 0)
	for _, fs := range perFile {
		findings = append(findings, fs...)
	}

	for _, rule := range opts.Rules {
		pr, ok := rule.(ProjectRule)
		if !ok {
			continue
		}
		extra := pr.CheckProject(ctx, ProjectInfo{
			PathRoot:     pathRoot,
			CoveragePath: opts.CoveragePath,
		})
		for _, f := range extra {
			if _, ok := contents[f.File]; !ok {
				if b, err := os.ReadFile(f.File); err == nil {
					contents[f.File] = b
				}
			}
		}
		findings = append(findings, extra...)
	}

	findings = FilterInlineIgnores(findings, contents)
	ApplySeverity(findings, opts.RuleSeverity)
	AssignFingerprints(findings, contents, pathRoot)
	RelativizeFindings(findings, pathRoot)

	SortFindings(findings)
	return Result{Findings: findings, Files: len(files)}, nil
}

func ruleDisabledForPath(ruleID, filePath, pathRoot string, overrides []PathOverride) bool {
	if len(overrides) == 0 {
		return false
	}
	rel := RelPath(filePath, pathRoot)
	base := filepath.Base(filePath)
	for _, o := range overrides {
		if o.Path == "" {
			continue
		}
		if !MatchGlob(o.Path, rel) && !MatchGlob(o.Path, base) {
			continue
		}
		for _, id := range o.Disable {
			if id == ruleID {
				return true
			}
		}
	}
	return false
}

// ApplySeverity overrides finding severity from config map[rule]severity.
func ApplySeverity(findings []Finding, sev map[string]string) {
	if len(sev) == 0 {
		return
	}
	for i := range findings {
		if s, ok := sev[findings[i].Rule]; ok && s != "" {
			findings[i].Severity = s
		}
	}
}

func startBatchPool(ctx context.Context, n int) chan *parse.Batch {
	if n < 1 {
		n = 1
	}
	pool := make(chan *parse.Batch, n)
	started := 0
	for i := 0; i < n; i++ {
		b, err := parse.StartBatch(ctx)
		if err != nil {
			continue
		}
		pool <- b
		started++
	}
	if started == 0 {
		return nil
	}
	return pool
}

func closeBatchPool(pool chan *parse.Batch) {
	if pool == nil {
		return
	}
	close(pool)
	for b := range pool {
		_ = b.Close()
	}
}

func anyNeedsAST(rules []Rule) bool {
	for _, r := range rules {
		if ar, ok := r.(ASTRule); ok && ar.NeedsAST() {
			return true
		}
	}
	return false
}

func withAST(ctx context.Context, file File, p parse.Parser, pool chan *parse.Batch) File {
	var model parse.Model
	var err error
	switch {
	case p != nil:
		model, err = p.Parse(ctx, file.Path, file.Content)
	case pool != nil:
		select {
		case b, ok := <-pool:
			if !ok || b == nil {
				model, err = parse.File(ctx, file.Path, file.Content)
				break
			}
			model, err = b.Parse(ctx, file.Path, file.Content)
			select {
			case pool <- b:
			case <-ctx.Done():
				_ = b.Close()
			}
		case <-ctx.Done():
			err = ctx.Err()
		}
	default:
		model, err = parse.File(ctx, file.Path, file.Content)
	}
	file.Model = model
	file.ModelErr = err
	file.ModelOK = true
	return file
}

func parseErrorFinding(file File) Finding {
	msg := "AST parse failed"
	if file.ModelErr != nil {
		if errors.Is(file.ModelErr, parse.ErrUnavailable) {
			msg = "AST helper unavailable: " + file.ModelErr.Error()
		} else {
			msg = fmt.Sprintf("AST parse error: %v", file.ModelErr)
		}
	}
	return Finding{
		File:     file.Path,
		Line:     1,
		Rule:     "parse-error",
		Severity: "note",
		Message:  msg,
	}
}

// SortFindings sorts by file, line, rule (determinism after parallel Run).
func SortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Rule < b.Rule
	})
}
