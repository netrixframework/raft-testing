package normal

import (
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
)

func ExpectLeader() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()).
			And(util.IsCorrectLeader()),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(util.CountVotes())

	testcase := testlib.NewTestCase(
		"ExpectLeader",
		1*time.Minute,
		sm,
		filters,
	)
	return testcase
}
