package pct

import (
	"time"

	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func MultiDrop() *testlib.TestCase {
	stateMachine := sm.NewStateMachine()
	filters := testlib.NewFilterSet()

	filters.AddFilter(
		testlib.If(
			sm.IsMessageSend().And(util.IsMessageType(raftpb.MsgAppResp).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		).Then(testlib.DropMessage()),
	)

	testCase := testlib.NewTestCase("MultiDrop", 2*time.Minute, stateMachine, filters)
	return testCase
}

func MultiDropProperty() *sm.StateMachine {
	property := sm.NewStateMachine()

	start := property.Builder()

	start.On(
		sm.IsMessageReceive().And(util.IsMessageType(raftpb.MsgAppResp).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		"AppRespReceived",
	)
	start.MarkSuccess()

	return property
}
