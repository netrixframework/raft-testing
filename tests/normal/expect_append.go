package additional

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func ExpectAppend() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	appendDelivered := init.On(
		sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgApp)),
		"AppendDelivered",
	)
	appendDelivered.On(
		sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgAppResp)).
			And(util.IsSenderSameAs("r")),
		sm.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(
			sm.IsMessageSend().
				And(util.IsMessageType(raftpb.MsgApp)),
		).Then(
			testlib.OnceAction("recordReceiver", util.RecordMessageReceiver("r")),
			testlib.DeliverMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"ExpectAppend",
		1*time.Minute,
		stateMachine,
		filters,
	)
	return testcase
}
