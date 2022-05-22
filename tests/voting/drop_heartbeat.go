package voting

import (
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// Drop heartbeat messages and Append entries messages
// to a particular replica and expect it to transition to candidate state

func DropHeartbeat() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		"LeaderElected",
	).On(
		testlib.IsEventOfF(util.RandomReplica()).
			And(util.IsStateChange()).
			And(util.IsStateCandidate()),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	// We need to ensure that the random replica we have picked does not become leader
	// If this happens, then the test will not succeed
	// Hence we drop all MsgVoteResp messages to it
	filters.AddFilter(
		testlib.If(
			testlib.IsMessageSend().
				And(testlib.IsMessageToF(util.RandomReplica())).
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
			sm.InState("LeaderElected").
				And(testlib.IsMessageSend()).
				And(testlib.IsMessageToF(util.RandomReplica())).
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
		sm,
		filters,
	)
	testcase.SetupFunc(util.PickRandomReplica())
	return testcase
}
