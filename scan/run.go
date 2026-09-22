package scan

import (
	"context"
	"errors"
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

type File struct {
	Path    string
	Content []byte
}

type Rule interface {
	ID() string
	Check(file File) []Finding
}

type Options struct {
	Rules []Rule
}

func Run(ctx context.Context, roots []string, opts Options) ([]Finding, error) {
	if opts.Rules == nil {
		return nil, ErrRulesRequired
	}
	files, err := Walk(ctx, roots)
	if err != nil {
		return nil, err
	}
	findings := []Finding{}
	for _, file := range files {
		for _, rule := range opts.Rules {
			findings = append(findings, rule.Check(file)...)
		}
	}
	return findings, nil
}
