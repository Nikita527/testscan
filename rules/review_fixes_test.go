package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestErrorContractAssert_StatusOnlyDefault(t *testing.T) {
	rule := rules.NewErrorContractAssert(rules.ErrorContractAssertOpts{})
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_bad", QualName: "test_bad", Lineno: 1,
				Asserts: []parse.Assert{{
					Text: "assert response.status_code == 400",
					Left: "response.status_code", Right: "400", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("want 1, got %v", got)
	}
}

func TestErrorContractAssert_StatusOnlyFalse_ErrorNamed(t *testing.T) {
	f := false
	rule := rules.NewErrorContractAssert(rules.ErrorContractAssertOpts{ErrorStatusOnly: &f})
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_validation_error_payload", QualName: "test_validation_error_payload", Lineno: 1,
				Asserts: []parse.Assert{{
					Text: `assert "email" in response.data["errors"][0]["detail"]`,
					Left: `response.data["errors"][0]["detail"]`, Right: `"email"`, Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("status-only=false should flag error-shaped test without code path, got %v", got)
	}
}

func TestErrorContractAssert_StatusOnlyFalse_HappyPathClean(t *testing.T) {
	f := false
	rule := rules.NewErrorContractAssert(rules.ErrorContractAssertOpts{ErrorStatusOnly: &f})
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_create_ok", QualName: "test_create_ok", Lineno: 1,
				Asserts: []parse.Assert{{
					Text: "assert response.status_code == 200",
					Left: "response.status_code", Right: "200", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("happy-path must stay clean when status-only=false, got %v", got)
	}
}

func TestRaisesWithoutCheck_UnrelatedGetattr(t *testing.T) {
	rule := rules.NewRaisesWithoutCheck(rules.RaisesWithoutCheckOpts{})
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Raises: []parse.Raise{{
					Exc: "DomainError", Lineno: 2, EndLineno: 4, AsName: "",
				}},
				Asserts: []parse.Assert{{
					Text: `assert getattr(payload, "code") == "x"`,
					Left: `getattr(payload, "code")`, Right: `"x"`, Lineno: 5,
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("unrelated getattr must not suppress finding, got %v", got)
	}
}

func TestRaisesWithoutCheck_ExcInfoGetattrClean(t *testing.T) {
	rule := rules.NewRaisesWithoutCheck(rules.RaisesWithoutCheckOpts{})
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Raises: []parse.Raise{{
					Exc: "DomainError", Lineno: 2, EndLineno: 4, AsName: "",
				}},
				Asserts: []parse.Assert{{
					Text: `assert getattr(exc_info.value, "code") == "x"`,
					Left: `getattr(exc_info.value, "code")`, Right: `"x"`, Lineno: 5,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("exc_info getattr must be clean, got %v", got)
	}
}

func TestRaisesWithoutCheck_ShortAsNameDoesNotSuppress(t *testing.T) {
	rule := rules.NewRaisesWithoutCheck(rules.RaisesWithoutCheckOpts{})
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Raises: []parse.Raise{{
					Exc: "DomainError", Lineno: 2, EndLineno: 4, AsName: "e",
				}},
				Asserts: []parse.Assert{{
					Text: `assert getattr(payload, "code") == "x"`,
					Left: `getattr(payload, "code")`, Right: `"x"`, Lineno: 5,
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("short as-name must not suppress via getattr Contains, got %v", got)
	}
}

func TestErrorContractAssert_StatusOnlyFalse_TokenCues(t *testing.T) {
	f := false
	rule := rules.NewErrorContractAssert(rules.ErrorContractAssertOpts{ErrorStatusOnly: &f})
	// "fails" must not match inside unrelated tokens via bare substring — use failures token.
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_profile_assignment", QualName: "test_profile_assignment", Lineno: 1,
				Asserts: []parse.Assert{{
					Text: "assert response.status_code == 200",
					Left: "response.status_code", Right: "200", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("non-error name must stay clean, got %v", got)
	}
	got = rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_create_failures_logged", QualName: "test_create_failures_logged", Lineno: 1,
				Asserts: []parse.Assert{{
					Text: "assert True",
					Left: "True", Right: "", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("failures token should gate error-shaped test, got %v", got)
	}
}

func TestNoAssert_PasswordPassesChecksStillFlags(t *testing.T) {
	rule := rules.NewNoAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_password_passes_checks", QualName: "test_password_passes_checks", Lineno: 1,
				Calls: []parse.Call{{Name: "run_password_flow", Lineno: 2, Bare: true}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("mid-token passes must still flag no-assert, got %v", got)
	}
}

func TestNoAssert_SwallowsMidClean(t *testing.T) {
	rule := rules.NewNoAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_release_swallows_redis_errors", QualName: "test_release_swallows_redis_errors", Lineno: 1,
				Calls: []parse.Call{{Name: "release_lock", Lineno: 2, Bare: true}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("swallows mid-token must be clean, got %v", got)
	}
}

func TestRBACForbiddenSignal_DigitBoundary(t *testing.T) {
	rule := rules.NewRBACMutationGuard(rules.RBACMutationGuardOpts{})
	// 1401 must not count as 401
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_rbac_update", QualName: "test_rbac_update", Lineno: 1,
				Calls: []parse.Call{{Name: "client.post", Lineno: 2}},
				Asserts: []parse.Assert{{
					Text: "assert response.status_code == 1401",
					Left: "response.status_code", Right: "1401", Lineno: 3,
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("1401 must not satisfy 401 signal, got %v", got)
	}

	got = rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_rbac_update", QualName: "test_rbac_update", Lineno: 1,
				Calls: []parse.Call{{Name: "client.post", Lineno: 2}},
				Asserts: []parse.Assert{{
					Text: "assert response.status_code == 401",
					Left: "response.status_code", Right: "401", Lineno: 3,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("401 must clean, got %v", got)
	}
}

func TestMissingMirror_SkipsVenvDir(t *testing.T) {
	root := t.TempDir()
	domain := filepath.Join(root, ".venv", "app", "orders", "domain")
	if err := os.MkdirAll(domain, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(domain, "pricing.py")
	if err := os.WriteFile(src, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := rules.NewMissingMirrorTest(rules.MissingMirrorTestOpts{
		SourceGlob:     "app/**/domain/*.py",
		MirrorTemplate: "tests/{x}/domain/test_{m}.py",
	})
	pr := rule.(scan.ProjectRule)
	got := pr.CheckProject(t.Context(), scan.ProjectInfo{
		PathRoot:         root,
		RespectGitignore: true,
	})
	if len(got) != 0 {
		t.Fatalf(".venv sources must be skipped, got %v", got)
	}
}
