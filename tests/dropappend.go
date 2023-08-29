package tests

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func DropAppendTest() *testlib.TestCase {
	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(
			sm.IsMessageSend().
				And(util.IsMessageType(raftpb.MsgApp)),
		).Then(
			testlib.OnceAction("recordIndex", util.RecordIndex("appIndex")),
			testlib.OnceAction("recordReceiver", util.RecordMessageReceiver("r")),
			testlib.OnceAction("dropMessage", testlib.DropMessage()),
		),
	)

	testcase := testlib.NewTestCase(
		"DropAppend",
		1*time.Minute,
		sm.NewStateMachine(),
		filters,
	)
	return testcase
}

func DropAppendProperty() *sm.StateMachine {
	stateMachine := sm.NewStateMachine()
	init := stateMachine.Builder()
	init.On(
		sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgApp)),
		"AppendObserved",
	).On(
		sm.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgApp)).
			And(util.IsReceiverSameAs("r")).
			And(util.IsSameIndex("appIndex")),
		sm.SuccessStateLabel,
	)
	return stateMachine
}
