package parse

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed ast_dump.py
var astDumpPy []byte

var (
	// ErrUnavailable means python/uv is missing or the helper failed to start.
	ErrUnavailable = errors.New("python AST helper unavailable")

	scriptOnce sync.Once
	scriptPath string
	scriptErr  error

	mu           sync.RWMutex
	activeParser Parser = &helperParser{}

	discoveryMu   sync.RWMutex
	pythonFuncs   []string
	pythonClasses []string
)

// SetDiscoveryPatterns configures AST test discovery globs (pytest-style).
// Empty slices keep helper defaults (test_* / Test*).
func SetDiscoveryPatterns(functions, classes []string) {
	discoveryMu.Lock()
	pythonFuncs = append([]string{}, functions...)
	pythonClasses = append([]string{}, classes...)
	discoveryMu.Unlock()
}

// DiscoveryPatterns returns the active discovery globs (empty = helper defaults).
func DiscoveryPatterns() (functions, classes []string) {
	return discoveryPatterns()
}

func discoveryPatterns() (funcs, classes []string) {
	discoveryMu.RLock()
	defer discoveryMu.RUnlock()
	return append([]string{}, pythonFuncs...), append([]string{}, pythonClasses...)
}

// Model is the JSON schema from ast_dump.py (per file).
//
// Per test: QualName, ClassName, Decorators, Fixtures,
// Asserts, Calls, Raises, TryExcept, Assignments, ForLoops, BodyNorm,
// IsEmpty, HasAssert, HasRaises.
// Collection walks classes and module-level functions (not a bare ast.walk by name).
// Imports lists every Import / ImportFrom node in the file.
type Model struct {
	Tests   []TestFunc `json:"tests"`
	Imports []Import   `json:"imports"`
}

// Import is an Import or ImportFrom AST node (file-level or nested).
type Import struct {
	Kind   string   `json:"kind"`   // "import" | "from"
	Module string   `json:"module"` // from-import module; empty for import
	Names  []string `json:"names"`  // imported names or dotted module paths
	Level  int      `json:"level"`  // ImportFrom relative level (0 = absolute)
	Lineno int      `json:"lineno"`
}

// TestFunc describes one test_* / Test* function.
type TestFunc struct {
	Name        string       `json:"name"`
	QualName    string       `json:"qualname"`
	ClassName   string       `json:"class_name"`
	Lineno      int          `json:"lineno"`
	EndLineno   int          `json:"end_lineno"`
	Decorators  []string     `json:"decorators"`
	Fixtures    []string     `json:"fixtures"`
	Asserts     []Assert     `json:"asserts"`
	Calls       []Call       `json:"calls"`
	Raises      []Raise      `json:"raises"`
	TryExcept   []TryExcept  `json:"try_except"`
	Assignments []Assignment `json:"assignments"`
	ForLoops    []ForLoop    `json:"for_loops"`
	BodyNorm    string       `json:"body_norm"`
	IsEmpty     bool         `json:"is_empty"`
	HasAssert   bool         `json:"has_assert"`
	HasRaises   bool         `json:"has_raises"`
}

// Assert kinds: compare | isinstance | truthy | mock_method | tuple | unittest_bool | other
type Assert struct {
	Kind        string `json:"kind"`
	Lineno      int    `json:"lineno"`
	Text        string `json:"text"`
	Left        string `json:"left"`
	Right       string `json:"right"`
	LeftIsCall  bool   `json:"left_is_call"`
	RightIsCall bool   `json:"right_is_call"`
}

type Call struct {
	Name   string `json:"name"`
	Lineno int    `json:"lineno"`
}

type Raise struct {
	Exc           string `json:"exc"`
	HasMatch      bool   `json:"has_match"`
	BodyStmtCount int    `json:"body_stmt_count"`
	Lineno        int    `json:"lineno"`
}

type TryExcept struct {
	Bare             bool `json:"bare"`
	CatchesException bool `json:"catches_exception"`
	HasRaise         bool `json:"has_raise"`
	Lineno           int  `json:"lineno"`
}

// Assignment is a .return_value / .side_effect assignment inside a test.
type Assignment struct {
	Target string `json:"target"`
	Value  string `json:"value"`
	Lineno int    `json:"lineno"`
}

// ForLoop is a for/async for; OnlyAsserts is true when the body is asserts only.
type ForLoop struct {
	Lineno      int  `json:"lineno"`
	EndLineno   int  `json:"end_lineno"`
	OnlyAsserts bool `json:"only_asserts"`
}

// Parser lets tests replace the AST helper (mock JSON without uv).
type Parser interface {
	Parse(ctx context.Context, path string, content []byte) (Model, error)
}

// SetParser replaces the global parser and returns a restore func.
//
// Deprecated: prefer scan.Options.Parser. Kept for one release for rule unit
// tests that call Check outside scan.Run.
func SetParser(p Parser) func() {
	mu.Lock()
	prev := activeParser
	activeParser = p
	mu.Unlock()
	return func() {
		mu.Lock()
		activeParser = prev
		mu.Unlock()
	}
}

// File parses path/content via the AST helper (or the active Parser).
func File(ctx context.Context, path string, content []byte) (Model, error) {
	mu.RLock()
	p := activeParser
	mu.RUnlock()
	return p.Parse(ctx, path, content)
}

type helperParser struct{}

func (helperParser) Parse(ctx context.Context, path string, content []byte) (Model, error) {
	exe, prefix, err := findPython()
	if err != nil {
		return Model{}, err
	}
	script, err := ensuredScript()
	if err != nil {
		return Model{}, err
	}

	tmp, err := os.CreateTemp("", "testscan-*.py")
	if err != nil {
		return Model{}, err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return Model{}, err
	}
	if err := tmp.Close(); err != nil {
		return Model{}, err
	}

	args := append(append([]string{}, prefix...), script, tmpPath)
	cmd := exec.CommandContext(ctx, exe, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Model{}, ctx.Err()
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return Model{}, fmt.Errorf("ast helper exit %d: %s", ee.ExitCode(), bytes.TrimSpace(stderr.Bytes()))
		}
		return Model{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	var model Model
	if err := json.Unmarshal(stdout.Bytes(), &model); err != nil {
		return Model{}, fmt.Errorf("ast helper JSON: %w", err)
	}
	normalizeModel(&model)
	_ = path
	return model, nil
}

func normalizeModel(model *Model) {
	if model.Tests == nil {
		model.Tests = []TestFunc{}
	}
	for i := range model.Tests {
		t := &model.Tests[i]
		if t.QualName == "" {
			if t.ClassName != "" {
				t.QualName = t.ClassName + "." + t.Name
			} else {
				t.QualName = t.Name
			}
		}
		if t.Asserts == nil {
			t.Asserts = []Assert{}
		}
		if t.Calls == nil {
			t.Calls = []Call{}
		}
		if t.Raises == nil {
			t.Raises = []Raise{}
		}
		if t.TryExcept == nil {
			t.TryExcept = []TryExcept{}
		}
		if t.Decorators == nil {
			t.Decorators = []string{}
		}
		if t.Fixtures == nil {
			t.Fixtures = []string{}
		}
	}
}

func findPython() (exe string, prefix []string, err error) {
	if uv, e := exec.LookPath("uv"); e == nil {
		return uv, []string{"run", "--no-project", "python"}, nil
	}
	for _, name := range []string{"python3", "python"} {
		if p, e := exec.LookPath(name); e == nil {
			return p, nil, nil
		}
	}
	return "", nil, ErrUnavailable
}

func ensuredScript() (string, error) {
	scriptOnce.Do(func() {
		dir, err := os.MkdirTemp("", "testscan-parse-")
		if err != nil {
			scriptErr = err
			return
		}
		path := filepath.Join(dir, "ast_dump.py")
		if err := os.WriteFile(path, astDumpPy, 0o644); err != nil {
			scriptErr = err
			return
		}
		scriptPath = path
	})
	return scriptPath, scriptErr
}

// StaticParser returns a fixed model (for rule unit tests).
type StaticParser struct {
	Model Model
	Err   error
}

func (s StaticParser) Parse(context.Context, string, []byte) (Model, error) {
	return s.Model, s.Err
}

// batchReq / batchResp are JSONL framing for the long-lived helper.
type batchReq struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	// Always sent (even empty) so the helper can reset to pytest defaults.
	PythonFunctions []string `json:"python_functions"`
	PythonClasses   []string `json:"python_classes"`
}

type batchResp struct {
	Path  string `json:"path"`
	OK    bool   `json:"ok"`
	Model Model  `json:"model"`
	Error string `json:"error"`
}

// Batch is one long-lived Python process (stdin JSONL → stdout JSONL).
// Safe for concurrent use (mutex around request/response).
type Batch struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	stderr bytes.Buffer
}

// StartBatch starts the helper in batch mode (no file argv).
func StartBatch(ctx context.Context) (*Batch, error) {
	exe, prefix, err := findPython()
	if err != nil {
		return nil, err
	}
	script, err := ensuredScript()
	if err != nil {
		return nil, err
	}
	args := append(append([]string{}, prefix...), script)
	cmd := exec.CommandContext(ctx, exe, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	b := &Batch{cmd: cmd, stdin: stdin}
	cmd.Stderr = &b.stderr
	sc := bufio.NewScanner(stdout)
	// models can be large
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 16*1024*1024)
	b.stdout = sc
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return b, nil
}

// Parse sends one JSONL request and reads one JSONL response (no tempfile).
func (b *Batch) Parse(ctx context.Context, path string, content []byte) (Model, error) {
	if b == nil {
		return Model{}, ErrUnavailable
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return Model{}, err
	}

	funcs, classes := discoveryPatterns()
	req, err := json.Marshal(batchReq{
		Path:            path,
		Source:          string(content),
		PythonFunctions: funcs,
		PythonClasses:   classes,
	})
	if err != nil {
		return Model{}, err
	}
	req = append(req, '\n')
	if _, err := b.stdin.Write(req); err != nil {
		return Model{}, fmt.Errorf("%w: write: %v", ErrUnavailable, err)
	}

	if !b.stdout.Scan() {
		if err := b.stdout.Err(); err != nil {
			return Model{}, fmt.Errorf("%w: read: %v", ErrUnavailable, err)
		}
		return Model{}, fmt.Errorf("%w: batch EOF (%s)", ErrUnavailable, bytes.TrimSpace(b.stderr.Bytes()))
	}
	var resp batchResp
	if err := json.Unmarshal(b.stdout.Bytes(), &resp); err != nil {
		return Model{}, fmt.Errorf("ast helper JSON: %w", err)
	}
	if !resp.OK {
		msg := resp.Error
		if msg == "" {
			msg = "parse failed"
		}
		return Model{}, fmt.Errorf("ast helper: %s", msg)
	}
	normalizeModel(&resp.Model)
	return resp.Model, nil
}

// Close closes stdin and waits for the helper to exit.
func (b *Batch) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stdin != nil {
		_ = b.stdin.Close()
		b.stdin = nil
	}
	if b.cmd != nil && b.cmd.Process != nil {
		err := b.cmd.Wait()
		b.cmd = nil
		return err
	}
	return nil
}

// BatchParser adapts Batch to Parser (shared session).
type BatchParser struct {
	Batch *Batch
}

func (p BatchParser) Parse(ctx context.Context, path string, content []byte) (Model, error) {
	return p.Batch.Parse(ctx, path, content)
}
