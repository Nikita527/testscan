package scan

import "testing"

func TestBatchPoolSize(t *testing.T) {
	cases := []struct {
		workers, nFiles, want int
	}{
		{16, 1, 1},
		{16, 100, 16},
		{4, 0, 1},
		{1, 10, 1},
		{8, 8, 8},
		{8, 3, 3},
	}
	for _, tc := range cases {
		if got := batchPoolSize(tc.workers, tc.nFiles); got != tc.want {
			t.Errorf("batchPoolSize(%d, %d)=%d, want %d", tc.workers, tc.nFiles, got, tc.want)
		}
	}
}
