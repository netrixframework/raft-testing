package tests

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func DropVotesTest() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(
			util.IsMessageType(raftpb.MsgVote).And(sm.IsMessageFrom(types.ReplicaID("4"))),
		).Then(testlib.DropMessage()),
	)

	testCase := testlib.NewTestCase("DropVotes", 2*time.Minute, stateMachine, filters)

	return testCase
}

func DropVotesProperty() *sm.StateMachine {
	property := sm.NewStateMachine()

	start := property.Builder()

	start.On(
		sm.IsMessageReceive().And(util.IsMessageType(raftpb.MsgVoteResp).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		"VoteDelivered",
	)

	start.On(
		sm.IsMessageSend().And(util.IsMessageType(raftpb.MsgApp).And(sm.IsMessageTo(types.ReplicaID("4")))),
		sm.SuccessStateLabel,
	)

	return property
}
