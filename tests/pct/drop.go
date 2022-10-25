package pct

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func DropVote() *testlib.TestCase {
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

func DropVoteProperty() *sm.StateMachine {
	property := sm.NewStateMachine()

	start := property.Builder()

	start.On(
		sm.IsMessageReceive().And(util.IsMessageType(raftpb.MsgVote).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		sm.FailStateLabel,
	)

	start.On(
		sm.IsMessageReceive().And(util.IsMessageType(raftpb.MsgHeartbeatResp).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		sm.SuccessStateLabel,
	)

	return property
}
