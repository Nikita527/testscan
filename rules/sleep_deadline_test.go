package rules_test

import (
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/rules"
)

func TestSleepInTest_RegressionDoesNotSuppressRealSleeps(t *testing.T) {
	rule := rules.NewSleepInTest()
	cases := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "fixed_sleep_with_return",
			src:  "import time\ndef test_x():\n    time.sleep(1)\n    assert ready()\n    return\n",
			want: 1,
		},
		{
			name: "timeout_variable_not_deadline",
			src:  "import time\ndef test_x():\n    timeout = 5\n    time.sleep(1)\n    assert ready()\n",
			want: 1,
		},
		{
			name: "while_break_without_deadline",
			src:  "import time\ndef test_x():\n    while not ready():\n        time.sleep(0.1)\n        if tries() > 10:\n            break\n    assert ready()\n",
			want: 1,
		},
		{
			name: "sleep_zero_clean",
			src:  "import asyncio\nasync def test_x():\n    await asyncio.sleep(0)\n    assert True\n",
			want: 0,
		},
		{
			name: "fail_after_poll_clean",
			src: "import asyncio\nimport anyio\nasync def test_x():\n" +
				"    async with anyio.fail_after(1.0):\n" +
				"        while True:\n" +
				"            if ready():\n" +
				"                break\n" +
				"            await asyncio.sleep(0.05)\n" +
				"    assert ready()\n",
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "test_case.py"), tc.src)
			res, err := testRun(t, []string{dir}, rule)
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for _, f := range res.Findings {
				if f.Rule == "sleep-in-test" {
					n++
				}
			}
			if n != tc.want {
				t.Fatalf("want %d sleep-in-test, got %d (%+v)", tc.want, n, res.Findings)
			}
		})
	}
}
