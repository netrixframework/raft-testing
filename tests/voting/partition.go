package voting

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
)

// 1. Partition one replica from the remaining
// 2. Wait for it to reach term 10
// 3. Heal partition and expect a remaining replica to reach term 10

func Partition() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	init.On(
		sm.IsEventOfF(util.RandomReplica()).
			And(util.IsNewTerm()).
			And(util.IsTerm(10)),
		"ReachedTermTen",
	).On(
		(sm.IsEventOfF(util.RandomReplica()).Not()).
			And(sm.IsEventType("TermChange")).
			And(util.IsTermGte(10)),
		sm.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(util.CountTerm())
	filters.AddFilter(
		testlib.If(
			stateMachine.InState(sm.StartStateLabel).
				And(
					sm.IsMessageFromF(util.RandomReplica()).
						Or(sm.IsMessageToF(util.RandomReplica())),
				),
		).Then(
			testlib.DropMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"Partition",
		1*time.Minute,
		stateMachine,
		filters,
	)
	testcase.SetupFunc(util.PickRandomReplica())
	return testcase
}
