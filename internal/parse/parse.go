package parse

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed ast_dump.py
var astDumpPy []byte

var (
	// ErrUnavailable — нет python/uv или helper не запустился.
	ErrUnavailable = errors.New("python AST helper unavailable")

	scriptOnce sync.Once
	scriptPath string
	scriptErr  error

	mu           sync.RWMutex
	activeParser Parser = &helperParser{}
)

// Model — минимальная схема JSON от ast_dump.py.
type Model struct {
	Tests []TestFunc `json:"tests"`
}

// TestFunc описывает одну test_*/Test* функцию.
type TestFunc struct {
	Name      string `json:"name"`
	Lineno    int    `json:"lineno"`
	EndLineno int    `json:"end_lineno"`
	IsEmpty   bool   `json:"is_empty"`
	HasAssert bool   `json:"has_assert"`
	HasRaises bool   `json:"has_raises"`
}

// Parser позволяет подменить helper в тестах (mock JSON без uv).
type Parser interface {
	Parse(ctx context.Context, path string, content []byte) (Model, error)
}

// SetParser подменяет парсер; возвращает restore.
// Для учёбы/тестов ок; в библиотечном API позже — Parser в scan.Options.
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

// File парсит path/content через AST-helper (или активный Parser).
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
		// LookPath ок, но запуск не удался → unavailable (fallback)
		return Model{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	var model Model
	if err := json.Unmarshal(stdout.Bytes(), &model); err != nil {
		return Model{}, fmt.Errorf("ast helper JSON: %w", err)
	}
	if model.Tests == nil {
		model.Tests = []TestFunc{}
	}
	_ = path // path retained for API; content is what we parse
	return model, nil
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

// StaticParser возвращает фиксированную модель (для unit-тестов правил).
type StaticParser struct {
	Model Model
	Err   error
}

func (s StaticParser) Parse(context.Context, string, []byte) (Model, error) {
	return s.Model, s.Err
}
