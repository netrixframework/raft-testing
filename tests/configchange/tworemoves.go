package configchange

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
)

func TwoRemoves() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	filters := testlib.NewFilterSet()

	testCase := testlib.NewTestCase("TwoRemoves", 1*time.Minute, stateMachine, filters)

	return testCase
}
