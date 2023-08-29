package tests

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func NoLivenessTest() *testlib.TestCase {

	stateMachine := sm.NewStateMachine()
	start := stateMachine.Builder()
	start.On(
		util.IsLeader(types.ReplicaID("4")),
		"inter1",
	)

	// .On(
	// 	sm.IsMessageReceive().
	// 		And(sm.IsMessageTo(types.ReplicaID("2"))).
	// 		And(sm.IsMessageFrom(types.ReplicaID("4"))).
	// 		And(util.IsMessageType(raftpb.MsgApp)),
	// 	"Final",
	// )

	// start.On(
	// 	sm.IsMessageReceive().
	// 		And(sm.IsMessageTo(types.ReplicaID("2"))).
	// 		And(sm.IsMessageFrom(types.ReplicaID("4"))).
	// 		And(util.IsMessageType(raftpb.MsgApp)),
	// 	"inter2",
	// ).On(
	// 	util.IsLeader(types.ReplicaID("4")),
	// 	"Final",
	// )

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.IsolateNode(types.ReplicaID("5")),
	)
	filters.AddFilter(
		testlib.If(
			stateMachine.InState("inter1").
				// Or(stateMachine.InState("Final")).
				And(sm.IsMessageBetween(types.ReplicaID("1"), types.ReplicaID("4")).Or(
					sm.IsMessageBetween(types.ReplicaID("3"), types.ReplicaID("4")),
				)),
		).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(
			stateMachine.InState(sm.StartStateLabel).
				And(util.IsMessageType(raftpb.MsgVote).Or(util.IsMessageType(raftpb.MsgPreVote))).
				And(sm.IsMessageFrom(types.ReplicaID("4")).Not()),
		).Then(testlib.DropMessage()),
	)

	// filters.AddFilter(
	// 	testlib.If(
	// 		stateMachine.InState("inter1").
	// 			And(sm.IsMessageTo(types.ReplicaID("2"))).
	// 			And(sm.IsMessageFrom(types.ReplicaID("1")).Or(sm.IsMessageFrom(types.ReplicaID("3")))),
	// 	).Then(

	// 		testlib.StoreInSet(sm.Set("TwoDelayed")),
	// 	),
	// )
	// filters.AddFilter(
	// 	testlib.If(
	// 		sm.OnceCondition("DeliverTwoDelayed", stateMachine.InState("Final").Or(stateMachine.InState("inter2"))),
	// 	).Then(
	// 		testlib.OnceAction("DeliverTwoDelayed", testlib.DeliverAllFromSet(sm.Set("TwoDelayed"))),
	// 	),
	// )

	return testlib.NewTestCase("LivenessBugPreVote", 15*time.Second, stateMachine, filters)
}

func NoLivenessProperty() *sm.StateMachine {
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
