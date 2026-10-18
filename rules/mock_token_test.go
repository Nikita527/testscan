package rules

import "testing"

func TestHasMockToken(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"mock", true},
		{"mocker", true},
		{"mocks", true},
		{"mock_repo", true},
		{"repo_mock", true},
		{"my_mock_obj", true},
		{"hammock", false},
		{"mockery", false},
		{"demock", false},
		{"locker", false},
		{"m", false},
	}
	for _, tc := range cases {
		if got := hasMockToken(tc.name); got != tc.want {
			t.Errorf("hasMockToken(%q)=%v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestLooksLikeMockName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"mock_repo", true},
		{"MagicMock", true},
		{"mockRepo", true},
		{"hammock", false},
		{"mockery", false},
	}
	for _, tc := range cases {
		if got := looksLikeMockName(tc.name); got != tc.want {
			t.Errorf("looksLikeMockName(%q)=%v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestHasStubToken(t *testing.T) {
	if !hasStubToken("db_stub") || hasStubToken("stubble") {
		t.Fatalf("stub token boundary failed")
	}
}
