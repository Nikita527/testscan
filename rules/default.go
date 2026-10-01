package rules

import "github.com/Nikita527/testscan/scan"

// Default returns the built-in rules that are enabled unless disabled.
func Default() []scan.Rule {
	return []scan.Rule{
		NewEmptyTest(),
		NewNoAssert(),
		NewAssertTrue(),
		NewMockOnlyAssert(),
		NewTodoTest(),
		NewDuplicateTestName(),
		NewOnlyHappyPath(),
		NewAssertEqualsSame(),
		NewSnapshotOnly(),
		NewOvermockedIO(),
		NewPrivateImport(),
		NewNoBehaviorChange(),
		NewFakeMockAssert(),
		NewAssertTuple(),
		NewBroadRaises(),
		NewSwallowedException(),
		NewAssertInEmptyableLoop(),
		NewWeakAssert(),
		NewMockTautology(),
		NewSleepInTest(),
		NewWallClockInTest(),
		NewSkipWithoutReason(),
		NewNearDuplicateTest(),
		NewNameBodyMismatch(),
		NewSelfPatchedSUT(),
		NewExpectedRecomputed(),
		NewCommentedAssert(),
		NewOverbroadEquality(),
	}
}

// Optional returns opt-in project rules (off by default; use --enable / enable = []).
func Optional() []scan.Rule {
	return []scan.Rule{
		NewErrorContractAssert(ErrorContractAssertOpts{}),
		NewRaisesWithoutCheck(RaisesWithoutCheckOpts{}),
		NewMissingMirrorTest(MissingMirrorTestOpts{}),
		NewRBACMutationGuard(RBACMutationGuardOpts{}),
	}
}

// All returns Default ∪ Optional.
func All() []scan.Rule {
	d := Default()
	o := Optional()
	out := make([]scan.Rule, 0, len(d)+len(o))
	out = append(out, d...)
	out = append(out, o...)
	return out
}
