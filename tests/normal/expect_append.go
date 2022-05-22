package normal

import (
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func ExpectAppend() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	appendDelivered := init.On(
		testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgApp)),
		"AppendDelivered",
	)
	appendDelivered.On(
		testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgAppResp)).
			And(util.IsSenderSameAs("r")),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(
			testlib.IsMessageSend().
				And(util.IsMessageType(raftpb.MsgApp)),
		).Then(
			testlib.OnceAction(util.RecordMessageReceiver("r")),
			testlib.DeliverMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"ExpectAppend",
		1*time.Minute,
		sm,
		filters,
	)
	return testcase
}
