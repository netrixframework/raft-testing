package cmd

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	raft "github.com/netrixframework/raft-testing/raft/protocol"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func LivenessBugOne() *testlib.TestCase {

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

func CountLeaderChanges() sm.Action {
	return func(e *types.Event, ctx *sm.Context) {
		if !e.IsGeneric() {
			return
		}
		ty := e.Type.(*types.GenericEventType)
		if ty.T != "StateChange" || ty.Params["new_state"] != raft.StateLeader.String() {
			return
		}
		if curLeader, ok := ctx.Vars.GetString("leader"); ok && curLeader != string(e.Replica) {
			ctx.Vars.Set("leader", string(e.Replica))
			if !ctx.Vars.Exists("leaderCount") {
				ctx.Vars.SetCounter("leaderCount")
			}
			counter, _ := ctx.Vars.GetCounter("leaderCount")
			counter.Incr()
		}
	}
}

// 1-2-3 are fully connected, 4-2 and 5-3 are the additional connections, this configuration with Prevote enabled should not lead to the liveness bug
func LivenessBugTwo() *testlib.TestCase {
	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(sm.IsMessageBetween(types.ReplicaID("4"), types.ReplicaID("1"))).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(sm.IsMessageBetween(types.ReplicaID("4"), types.ReplicaID("3"))).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(sm.IsMessageBetween(types.ReplicaID("4"), types.ReplicaID("5"))).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(sm.IsMessageBetween(types.ReplicaID("5"), types.ReplicaID("1"))).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(sm.IsMessageBetween(types.ReplicaID("5"), types.ReplicaID("2"))).Then(testlib.DropMessage()),
	)
	return testlib.NewTestCase("LivenessBugTwo", 15*time.Second, sm.NewStateMachine(), filters)
}

// 1-2-3 fully connected, 4-2, 5-3, 4-5 are the additional connections,
// Should not lead to liveness bug with Prevote enabled
func LivenessBugThree() *testlib.TestCase {
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
