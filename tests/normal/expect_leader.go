package additional

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
)

func ExpectLeader() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()).
			And(util.IsCorrectLeader()),
		sm.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(util.CountVotes())

	testcase := testlib.NewTestCase(
		"ExpectLeader",
		1*time.Minute,
		stateMachine,
		filters,
	)
	return testcase
}
