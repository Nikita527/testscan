package parse

import "testing"

func TestSetDiscoveryPatterns_Clears(t *testing.T) {
	t.Cleanup(func() { SetDiscoveryPatterns(nil, nil) })

	SetDiscoveryPatterns([]string{"check_*"}, []string{"Suite*"})
	f, c := DiscoveryPatterns()
	if len(f) != 1 || f[0] != "check_*" || len(c) != 1 || c[0] != "Suite*" {
		t.Fatalf("custom patterns: funcs=%v classes=%v", f, c)
	}

	SetDiscoveryPatterns(nil, nil)
	f, c = DiscoveryPatterns()
	if len(f) != 0 || len(c) != 0 {
		t.Fatalf("empty should clear, got funcs=%v classes=%v", f, c)
	}
}
