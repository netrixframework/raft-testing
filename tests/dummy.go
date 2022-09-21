package tests

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
)

func AllowAllTest() *testlib.TestCase {

	setup := func(c *testlib.Context) error {
		for _, replica := range c.ReplicaStore.Iter() {
			if err := util.SetKeyValue(replica, "hello", "world"); err == nil {
				break
			}
		}
		return nil
	}

	stateMachine := sm.NewStateMachine()

	filters := testlib.NewFilterSet()

	filters.AddFilter(
		testlib.If(sm.IsMessageSend()).Then(testlib.DeliverMessage()),
	)

	testcase := testlib.NewTestCase(
		"AllowAll",
		10*time.Second,
		stateMachine,
		filters,
	)
	testcase.SetupFunc(setup)
	return testcase
}
