package voting

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// Drop all votes and expect a new term

func DropVotesNewTerm() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	votingPhase := init.On(
		sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgVote)),
		"VotingPhase",
	)
	votingPhase.On(
		util.IsNewTerm(),
		sm.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(util.CountTerm())
	filters.AddFilter(
		testlib.If(
			stateMachine.InState("VotingPhase").
				And(sm.IsMessageSend()).
				And(util.IsMessageType(raftpb.MsgVoteResp)),
		).Then(
			testlib.DropMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"DropVotesNewTerm",
		1*time.Minute,
		stateMachine,
		filters,
	)
	return testcase
}
