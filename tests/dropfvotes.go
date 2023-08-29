package tests

import (
	"fmt"
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func votesByTermByReplica(e *types.Event, c *sm.Context) (string, bool) {
	m, ok := util.GetMessageFromEvent(e, c)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("_votesDropped_%d_%d", m.Term, m.To), false
}

func DropFVotesTest() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		sm.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(
			sm.IsMessageSend().
				And(util.IsMessageType(raftpb.MsgVoteResp)).
				And(sm.CountF(votesByTermByReplica).LtF(util.FReplicas())),
		).Then(
			testlib.DropMessage(),
			testlib.IncrCounter(sm.CountF(votesByTermByReplica)),
		),
	)

	testcase := testlib.NewTestCase(
		"DropFVotes",
		1*time.Minute,
		stateMachine,
		filters,
	)
	return testcase
}

func DropFVotesProperty() *sm.StateMachine {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		sm.SuccessStateLabel,
	)
	return stateMachine
}
