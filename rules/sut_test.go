package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const sutSrc = `import json

import requests
from app.service import process, Meta
from app import service as svc_mod
from .helpers import local_help
from tests.utils import util


def _rows(n):
    return n


def test_project():
    process(1)


def test_private():
    _rows(1)


def test_str_method():
    json_payload = json.dumps({})
    json_payload.encode("utf-8")


def test_literal():
    "x".join([])


def test_dto_argument():
    json.dumps({"a": Meta(1)})


def test_ctor_subject():
    Meta(1)


def test_third_party():
    requests.get("x")


def test_relative():
    local_help()


def test_tests_pkg():
    util()


def test_alias():
    svc_mod.process(1)


def test_instance():
    s = Meta(1)
    s.run()


def test_fixture(client):
    client.get("/")


def test_nested_import():
    import app.service
    app.service.process(1)


def test_mock_is_not_sut(mocker):
    mocker.patch("app.service.process")
    process(2)
`

func TestSUTCalls_Definition(t *testing.T) {
	root := sutProject(t)
	writeFile(t, filepath.Join(root, "tests", "helpers.py"), "def local_help():\n    pass\n")
	writeFile(t, filepath.Join(root, "tests", "utils.py"), "def util():\n    pass\n")
	path := filepath.Join(root, "tests", "test_sut.py")
	writeFile(t, path, sutSrc)

	model, err := parse.BatchParser{Batch: sharedBatch}.Parse(context.Background(), path, []byte(sutSrc))
	if err != nil {
		t.Fatal(err)
	}
	ctx := rules.NewSUTContext(scan.File{Path: path, ProjectRoot: root}, model)

	want := map[string][]string{
		"test_project":         {"process"},
		"test_private":         nil,
		"test_str_method":      nil,
		"test_literal":         nil,
		"test_dto_argument":    nil,
		"test_ctor_subject":    {"Meta"},
		"test_third_party":     nil,
		"test_relative":        nil,
		"test_tests_pkg":       nil,
		"test_alias":           {"svc_mod.process"},
		"test_instance":        {"Meta", "Meta.run"},
		"test_fixture":         nil,
		"test_nested_import":   {"app.service.process"},
		"test_mock_is_not_sut": {"process"},
	}
	for _, tf := range model.Tests {
		var got []string
		for _, c := range ctx.Calls(tf) {
			got = append(got, c.Resolved)
		}
		sort.Strings(got)
		w := want[tf.Name]
		sort.Strings(w)
		if len(got) == 0 && len(w) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s: SUT calls = %v, want %v", tf.Name, got, w)
		}
	}
}
