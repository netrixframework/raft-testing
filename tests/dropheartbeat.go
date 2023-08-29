package tests

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func DropHeartbeatTest() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		"LeaderElected",
	).On(
		sm.IsEventOfF(util.RandomReplica()).
			And(util.IsStateChange()).
			And(util.IsStateCandidate()),
		sm.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	// We need to ensure that the random replica we have picked does not become leader
	// If this happens, then the test will not succeed
	// Hence we drop all MsgVoteResp messages to it
	filters.AddFilter(
		testlib.If(
			sm.IsMessageSend().
				And(sm.IsMessageToF(util.RandomReplica())).
				And(util.IsMessageType(raftpb.MsgVoteResp)),
		).Then(
			testlib.DropMessage(),
		),
	)
	// Given that our randomly chosen replica is not going to be a leader
	// we can then drop Heartbeat, MsgSnap and MsgApp messages to it
	// after a leader has been elected
	filters.AddFilter(
		testlib.If(
			stateMachine.InState("LeaderElected").
				And(sm.IsMessageSend()).
				And(sm.IsMessageToF(util.RandomReplica())).
				And(util.IsMessageType(raftpb.MsgHeartbeat).
					Or(util.IsMessageType(raftpb.MsgApp)).
					Or(util.IsMessageType(raftpb.MsgSnap))),
		).Then(
			testlib.DropMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"DropHeartbeat",
		1*time.Minute,
		stateMachine,
		filters,
	)
	testcase.SetupFunc(util.PickRandomReplica())
	return testcase
}

func DropHeartbeatProperty() *sm.StateMachine {
	property := sm.NewStateMachine()
	init := property.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		"LeaderElected",
	).On(
		sm.IsEventOfF(util.RandomReplica()).
			And(util.IsStateChange()).
			And(util.IsStateCandidate()),
		sm.SuccessStateLabel,
	)
	return property
}
