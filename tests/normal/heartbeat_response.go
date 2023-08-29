package additional

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func HeartbeatResponse() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	leaderElected := init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		"LeaderElected",
	)
	leaderElected.On(
		sm.IsMessageSend().
			And(util.IsSenderSameAs("r")).
			And(util.IsMessageType(raftpb.MsgHeartbeatResp)),
		sm.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgHeartbeat)),
		).Then(
			testlib.OnceAction("recordReceiver", util.RecordMessageReceiver("r")),
			testlib.DeliverMessage(),
		),
	)

	testcase := testlib.NewTestCase(
		"HeartbeatResponse",
		1*time.Minute,
		stateMachine,
		filters,
	)
	return testcase
}
