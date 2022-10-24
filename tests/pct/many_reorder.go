package pct

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// ManyReorder returns a test where the votes from 4 are delayed until 4 becomes a follower
// In effect, the vote is delayed until all other votes, append entries are delivered

// This will fail when 4 is the leader

func ManyReorder() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	stateMachine.Builder().On(
		util.IsStateFollower().And(sm.IsEventOf(types.ReplicaID("4"))),
		"FourFollower",
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
			util.IsStateFollower().And(sm.IsEventOf(types.ReplicaID("4"))),
		).Then(
			testlib.DeliverAllFromSet(sm.Set("reorderedVote")),
			testlib.DeliverMessage(),
		),
	)

	testCase := testlib.NewTestCase("ManyReorder", 2*time.Minute, stateMachine, filters)
	return testCase
}

func ManyReorderProperty() *sm.StateMachine {
	property := sm.NewStateMachine()
	property.Builder().On(
		util.IsStateFollower().And(sm.IsEventOf(types.ReplicaID("4"))),
		"FourFollower",
	).On(
		sm.IsMessageReceive().And(util.IsMessageType(raftpb.MsgVoteResp).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		"VoteReceived",
	).MarkSuccess()
	return property
}
