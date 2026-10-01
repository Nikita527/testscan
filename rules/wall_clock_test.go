package rules_test

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

const wallClockSrc = `from datetime import date, datetime, timezone

import pytest

from app.models import Env
from app.resolver import resolve


def test_model_field_via_name():
    today = datetime.now().date()
    Env.objects.create(valid_from=today - timedelta(days=1))


def test_assert_direct():
    assert date.today().year > 2000


def test_aware_positional():
    Env.objects.create(at=datetime.now(timezone.utc))


def test_aware_kw():
    Env.objects.create(at=datetime.now(tz=timezone.utc))


def test_aware_chain():
    Env.objects.create(at=datetime.now().astimezone())


def test_unused_value():
    now = datetime.now()
    print("x")


def test_pinned_missing_returns_error():
    result = resolve(as_of=date.today())
    assert result.code == "NOT_FOUND"


def test_error_in_raises_block():
    with pytest.raises(ValueError):
        resolve(as_of=date.today())


def test_call_arg_positive():
    result = resolve(as_of=date.today())
    assert result.ok


def test_utcnow_assert():
    assert datetime.utcnow().year > 2000
`

func wallClockProject(t *testing.T, aware bool) string {
	t.Helper()
	root := t.TempDir()
	if aware {
		writeFile(t, filepath.Join(root, "app", "views.py"),
			"from django.utils import timezone\n\n\ndef today():\n    return timezone.localdate()\n")
	} else {
		writeFile(t, filepath.Join(root, "app", "views.py"), "def today():\n    return 1\n")
	}
	writeFile(t, filepath.Join(root, "tests", "test_clock.py"), wallClockSrc)
	return root
}

func wallClockByTest(t *testing.T, root string) map[string]scan.Finding {
	t.Helper()
	res, err := testRunOpts(t, []string{filepath.Join(root, "tests")}, scan.Options{
		Rules:    []scan.Rule{rules.NewWallClockInTest()},
		PathRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]scan.Finding{}
	for _, f := range res.Findings {
		out[f.QualName] = f
	}
	return out
}

func TestWallClock_TZAwareProject(t *testing.T) {
	got := wallClockByTest(t, wallClockProject(t, true))
	var names []string
	for k, f := range got {
		names = append(names, k)
		if f.Severity != "warning" {
			t.Errorf("%s: severity %q, want warning", k, f.Severity)
		}
		if !strings.Contains(f.Message, "timezone-aware") || !strings.Contains(f.Message, "timezone.localdate()") {
			t.Errorf("%s: message %q", k, f.Message)
		}
	}
	sort.Strings(names)
	want := []string{
		"test_assert_direct", "test_call_arg_positive",
		"test_model_field_via_name", "test_utcnow_assert",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("reported %v, want %v", names, want)
	}
	if m := got["test_assert_direct"].Message; !strings.Contains(m, "`date.today()`") {
		t.Errorf("message should name the call: %q", m)
	}
}

func TestWallClock_NonAwareProjectKeepsNote(t *testing.T) {
	got := wallClockByTest(t, wallClockProject(t, false))
	for k, f := range got {
		if f.Severity != "note" {
			t.Errorf("%s: severity %q, want note", k, f.Severity)
		}
	}
	// aware calls are excluded in every project; everything naive stays a note.
	for _, aware := range []string{"test_aware_positional", "test_aware_kw", "test_aware_chain"} {
		if _, ok := got[aware]; ok {
			t.Errorf("%s: tz-aware call must not be reported", aware)
		}
	}
	for _, naive := range []string{"test_unused_value", "test_pinned_missing_returns_error", "test_model_field_via_name"} {
		if _, ok := got[naive]; !ok {
			t.Errorf("%s: naive call should stay a note in non-aware project", naive)
		}
	}
}

func TestWallClock_SettingsUseTZ(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pyproject.toml"),
		"[tool.pytest.ini_options]\nDJANGO_SETTINGS_MODULE = \"app.core.settings.test\"\n")
	writeFile(t, filepath.Join(root, "app", "core", "settings", "base.py"), "USE_TZ = True\n")
	writeFile(t, filepath.Join(root, "app", "core", "settings", "test.py"), "from .base import *\n")
	writeFile(t, filepath.Join(root, "tests", "test_clock.py"), wallClockSrc)
	got := wallClockByTest(t, root)
	f, ok := got["test_assert_direct"]
	if !ok || f.Severity != "warning" || !strings.Contains(f.Message, "USE_TZ = True") {
		t.Fatalf("settings USE_TZ must make project aware: %+v", got)
	}
}
