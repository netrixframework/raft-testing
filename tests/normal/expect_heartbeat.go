package normal

import (
	"time"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func ExpectHeartbeat() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	init.On(
		util.IsStateChange().
			And(util.IsStateLeader()),
		"LeaderElected",
	).On(
		testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgHeartbeat)),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()

	testcase := testlib.NewTestCase(
		"ExpectHeartbeat",
		1*time.Minute,
		sm,
		filters,
	)
	return testcase
}
