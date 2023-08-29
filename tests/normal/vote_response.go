package additional

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func VoteResponse() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	voteSent := init.On(
		sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgVote)),
		"VoteSent",
	)
	voteSent.On(
		sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgVoteResp)).
			And(util.IsSenderSameAs("r")),
		sm.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgVote)),
		).Then(
			testlib.OnceAction("recordReceiver", util.RecordMessageReceiver("r")),
			testlib.DeliverMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"VoteResponse",
		1*time.Minute,
		stateMachine,
		filters,
	)
	return testcase
}
