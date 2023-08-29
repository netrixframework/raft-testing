package tests

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func ReVoteTest() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	stateMachine.Builder().On(
		util.IsStateLeader(),
		"LeaderElected",
	).MarkSuccess()

	filters := testlib.NewFilterSet()

	filters.AddFilter(
		testlib.If(
			stateMachine.InState(sm.StartStateLabel).And(
				util.IsMessageType(raftpb.MsgVoteResp).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		).Then(
			testlib.StoreInSet(sm.Set("reorderedVote")),
		),
	)
	filters.AddFilter(
		testlib.If(
			util.IsStateLeader(),
		).Then(
			testlib.DeliverAllFromSet(sm.Set("reorderedVote")),
			testlib.DeliverMessage(),
		),
	)

	testCase := testlib.NewTestCase("SimpleReorder", 2*time.Minute, stateMachine, filters)
	return testCase
}

func ReVoteProperty() *sm.StateMachine {
	property := sm.NewStateMachine()
	property.Builder().On(
		util.IsStateLeader(),
		"LeaderElected",
	).On(
		sm.IsMessageReceive().And(util.IsMessageType(raftpb.MsgVoteResp).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		"VoteReceived",
	).MarkSuccess()
	return property
}
