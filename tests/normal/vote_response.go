package normal

import (
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func VoteResponse() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	voteSent := init.On(
		testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgVote)),
		"VoteSent",
	)
	voteSent.On(
		testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgVoteResp)).
			And(util.IsSenderSameAs("r")),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgVote)),
		).Then(
			testlib.OnceAction(util.RecordMessageReceiver("r")),
			testlib.DeliverMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"VoteResponse",
		1*time.Minute,
		sm,
		filters,
	)
	return testcase
}
