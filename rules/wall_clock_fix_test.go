package rules

import (
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
)

func TestWallClockAwareFix_DateChain(t *testing.T) {
	src := []byte("def test_x():\n    today = datetime.now().date()\n    stamp = datetime.now()\n")
	got := wallClockAwareFix(src, parse.Call{Name: "datetime.now", Lineno: 2})
	if want := "replace `datetime.now().date()` with `timezone.localdate()`, or freeze time"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	got = wallClockAwareFix(src, parse.Call{Name: "datetime.now", Lineno: 3})
	if want := "replace `datetime.now()` with `timezone.now()`, or freeze time"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestScanTimezoneUse_FromImportNames(t *testing.T) {
	cases := []struct {
		name, src       string
		wantLD, wantNow bool
	}{
		{"now only", "from django.utils.timezone import now\n", false, true},
		{"localdate only", "from django.utils.timezone import localdate\n", true, false},
		{"both", "from django.utils.timezone import now, localdate as ld\n", true, true},
		{"parenthesised", "from django.utils.timezone import (\n    localdate,\n    make_aware,\n)\n", true, false},
		{"unrelated name", "from django.utils.timezone import make_aware\n", false, false},
		{"module attr", "from django.utils import timezone\nx = timezone.localdate()\n", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ld, now := scanTimezoneUse([]byte(tc.src))
			if ld != tc.wantLD || now != tc.wantNow {
				t.Fatalf("got localdate=%v now=%v, want %v/%v", ld, now, tc.wantLD, tc.wantNow)
			}
		})
	}
}
