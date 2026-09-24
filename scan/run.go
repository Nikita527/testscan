package scan

import (
	"context"
	"errors"
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
}

// File — исходник теста. После Walk; в Run один раз заполняется AST (Model*),
// чтобы несколько AST-правил не спавнили Python повторно.
type File struct {
	Path    string
	Content []byte

	// ModelOK — Run уже вызывал parse.File для этого файла.
	ModelOK  bool
	Model    parse.Model
	ModelErr error
}

type Rule interface {
	ID() string
	Check(file File) []Finding
}

type Options struct {
	Rules   []Rule
	Workers int // 0 → runtime.NumCPU()
}

func Run(ctx context.Context, roots []string, opts Options) ([]Finding, error) {
	if opts.Rules == nil {
		return nil, ErrRulesRequired
	}
	files, err := Walk(ctx, roots)
	if err != nil {
		return nil, err
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	perFile := make([][]Finding, len(files))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	for i, file := range files {
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			file = withAST(gctx, file)
			var fs []Finding
			for _, rule := range opts.Rules {
				fs = append(fs, rule.Check(file)...)
			}
			perFile[i] = fs
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	findings := make([]Finding, 0)
	for _, fs := range perFile {
		findings = append(findings, fs...)
	}
	SortFindings(findings)
	return findings, nil
}

func withAST(ctx context.Context, file File) File {
	model, err := parse.File(ctx, file.Path, file.Content)
	file.Model = model
	file.ModelErr = err
	file.ModelOK = true
	return file
}

// SortFindings сортирует по file, line, rule (детерминизм после параллельного Run).
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
