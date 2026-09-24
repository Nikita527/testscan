package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config — настройки проекта из .testscan.toml или [tool.testscan].
type Config struct {
	FailOn  string // error|warning|never; пусто = не задано
	Disable []string
	Paths   []string
	Workers int    // 0 = не задано (NumCPU в scan)
	Source  string // путь файла, из которого загружено; пусто если нет
}

type fileTOML struct {
	FailOnKebab string   `toml:"fail-on"`
	FailOnSnake string   `toml:"fail_on"`
	Disable     []string `toml:"disable"`
	Paths       []string `toml:"paths"`
	Workers     int      `toml:"workers"`
}

type pyprojectTOML struct {
	Tool struct {
		Testscan fileTOML `toml:"testscan"`
	} `toml:"tool"`
}

// Load ищет конфиг от startDir вверх: сначала .testscan.toml, иначе pyproject.toml
// с [tool.testscan]. Отсутствие конфига — пустой Config без ошибки.
func Load(startDir string) (Config, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return Config{}, err
	}
	for {
		dot := filepath.Join(dir, ".testscan.toml")
		if st, err := os.Stat(dot); err == nil && !st.IsDir() {
			cfg, err := parseDotFile(dot)
			if err != nil {
				return Config{}, err
			}
			return cfg, nil
		}

		py := filepath.Join(dir, "pyproject.toml")
		if st, err := os.Stat(py); err == nil && !st.IsDir() {
			cfg, found, err := parsePyproject(py)
			if err != nil {
				return Config{}, err
			}
			if found {
				return cfg, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, nil
		}
		dir = parent
	}
}

func parseDotFile(path string) (Config, error) {
	var raw fileTOML
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	cfg, err := fromTOML(raw, path)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parsePyproject(path string) (Config, bool, error) {
	var raw pyprojectTOML
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return Config{}, false, fmt.Errorf("config %s: %w", path, err)
	}
	t := raw.Tool.Testscan
	if t.FailOnKebab == "" && t.FailOnSnake == "" &&
		len(t.Disable) == 0 && len(t.Paths) == 0 && t.Workers == 0 {
		return Config{}, false, nil
	}
	cfg, err := fromTOML(t, path)
	if err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}

func fromTOML(raw fileTOML, source string) (Config, error) {
	failOn := raw.FailOnKebab
	if failOn == "" {
		failOn = raw.FailOnSnake
	}
	if failOn != "" && failOn != "error" && failOn != "warning" && failOn != "never" {
		return Config{}, fmt.Errorf("config %s: invalid fail-on %q (want error|warning|never)", source, failOn)
	}
	if raw.Workers < 0 {
		return Config{}, fmt.Errorf("config %s: workers must be >= 0", source)
	}
	disable := raw.Disable
	if disable == nil {
		disable = []string{}
	}
	paths := raw.Paths
	if paths == nil {
		paths = []string{}
	}
	// paths в конфиге — относительно каталога файла конфига, не cwd
	base := filepath.Dir(source)
	resolved := make([]string, len(paths))
	for i, p := range paths {
		if p == "" || filepath.IsAbs(p) {
			resolved[i] = p
			continue
		}
		resolved[i] = filepath.Clean(filepath.Join(base, p))
	}
	return Config{
		FailOn:  failOn,
		Disable: disable,
		Paths:   resolved,
		Workers: raw.Workers,
		Source:  source,
	}, nil
}
