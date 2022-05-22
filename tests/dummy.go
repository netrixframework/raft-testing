package tests

import (
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
)

func AllowAllTest() *testlib.TestCase {

	setup := func(c *testlib.Context) error {
		for _, replica := range c.Replicas.Iter() {
			if err := util.SetKeyValue(replica, "hello", "world"); err == nil {
				break
			}
		}
		return nil
	}

	sm := testlib.NewStateMachine()

	filters := testlib.NewFilterSet()

	filters.AddFilter(
		testlib.If(testlib.IsMessageSend()).Then(testlib.DeliverMessage()),
	)

	testcase := testlib.NewTestCase(
		"AllowAll",
		10*time.Second,
		sm,
		filters,
	)
	testcase.SetupFunc(setup)
	return testcase
}
