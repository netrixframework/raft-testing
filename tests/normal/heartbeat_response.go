package normal

import (
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func HeartbeatResponse() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	leaderElected := init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		"LeaderElected",
	)
	leaderElected.On(
		testlib.IsMessageSend().
			And(util.IsSenderSameAs("r")).
			And(util.IsMessageType(raftpb.MsgHeartbeatResp)),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgHeartbeat)),
		).Then(
			testlib.OnceAction(util.RecordMessageReceiver("r")),
			testlib.DeliverMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"HeartbeatResponse",
		1*time.Minute,
		sm,
		filters,
	)
	return testcase
}
