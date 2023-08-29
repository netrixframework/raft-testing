package additional

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func ExpectHeartbeat() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		"LeaderElected",
	).On(
		sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgHeartbeat)),
		sm.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()

	testcase := testlib.NewTestCase(
		"ExpectHeartbeat",
		1*time.Minute,
		stateMachine,
		filters,
	)
	return testcase
}
