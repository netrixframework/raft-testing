package voting

import (
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// Drop all votes and expect a new term

func DropVotesNewTerm() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	votingPhase := init.On(
		testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgVote)),
		"VotingPhase",
	)
	votingPhase.On(
		util.IsNewTerm(),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(util.CountTerm())
	filters.AddFilter(
		testlib.If(
			sm.InState("VotingPhase").
				And(testlib.IsMessageSend()).
				And(util.IsMessageType(raftpb.MsgVoteResp)),
		).Then(
			testlib.DropMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"DropVotesNewTerm",
		1*time.Minute,
		sm,
		filters,
	)
	return testcase
}
