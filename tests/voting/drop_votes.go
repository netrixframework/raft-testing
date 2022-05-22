package voting

import (
	"fmt"
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// Test if the leader is elected despite dropping $f$ vote response messages

func votesByTermByReplica(e *types.Event, c *testlib.Context) (string, bool) {
	m, ok := util.GetMessageFromEvent(e, c)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("_votesDropped_%d_%d", m.Term, m.To), false
}

func DropVotes() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(
			testlib.IsMessageSend().
				And(util.IsMessageType(raftpb.MsgVoteResp)).
				And(testlib.CountF(votesByTermByReplica).LtF(util.FReplicas())),
		).Then(
			testlib.DropMessage(),
			testlib.CountF(votesByTermByReplica).Incr(),
		),
	)

	testcase := testlib.NewTestCase(
		"DropFVotes",
		1*time.Minute,
		sm,
		filters,
	)
	return testcase
}
