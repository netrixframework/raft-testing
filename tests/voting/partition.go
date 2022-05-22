package voting

import (
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
)

// 1. Partition one replica from the remaining
// 2. Wait for it to reach term 10
// 3. Heal partition and expect a remaining replica to reach term 10

func Partition() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	init.On(
		testlib.IsEventOfF(util.RandomReplica()).
			And(util.IsNewTerm()).
			And(util.IsTerm(10)),
		"ReachedTermTen",
	).On(
		(testlib.IsEventOfF(util.RandomReplica()).Not()).
			And(testlib.IsEventType("TermChange")).
			And(util.IsTermGte(10)),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(util.CountTerm())
	filters.AddFilter(
		testlib.If(
			sm.InState(testlib.StartStateLabel).
				And(
					testlib.IsMessageFromF(util.RandomReplica()).
						Or(testlib.IsMessageToF(util.RandomReplica())),
				),
		).Then(
			testlib.DropMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"Partition",
		1*time.Minute,
		sm,
		filters,
	)
	testcase.SetupFunc(util.PickRandomReplica())
	return testcase
}
