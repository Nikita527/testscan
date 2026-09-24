package parse_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
)

func TestStaticParser(t *testing.T) {
	p := parse.StaticParser{
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name:    "test_x",
				Lineno:  2,
				IsEmpty: true,
			}},
		},
	}
	got, err := p.Parse(context.Background(), "x.py", []byte("ignored"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tests) != 1 || !got.Tests[0].IsEmpty || got.Tests[0].Lineno != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestStaticParser_Error(t *testing.T) {
	p := parse.StaticParser{Err: errors.New("boom")}
	_, err := p.Parse(context.Background(), "x.py", nil)
	if err == nil {
		t.Fatal("want error")
	}
}
