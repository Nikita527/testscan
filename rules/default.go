package rules

import "github.com/Nikita527/testscan/scan"

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
	}
}
