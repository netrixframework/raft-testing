package tests

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
)

func LivenessNoCQTest() *testlib.TestCase {
	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(sm.IsMessageBetween(types.ReplicaID("4"), types.ReplicaID("1"))).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(sm.IsMessageBetween(types.ReplicaID("4"), types.ReplicaID("3"))).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(sm.IsMessageBetween(types.ReplicaID("5"), types.ReplicaID("1"))).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(sm.IsMessageBetween(types.ReplicaID("5"), types.ReplicaID("2"))).Then(testlib.DropMessage()),
	)
	return testlib.NewTestCase("LivenessBugThree", 15*time.Second, sm.NewStateMachine(), filters)
}

func LivenessNoCQProperty() *sm.StateMachine {
	property := sm.NewStateMachine()
	start := property.Builder()
	start.On(
		sm.ConditionWithAction(util.IsStateLeader(), util.CountLeaderChanges()),
		sm.StartStateLabel,
	)
	start.On(
		sm.Count("leaderCount").Gt(4),
		sm.FailStateLabel,
	)
	start.MarkSuccess()
	return property
}
