package appends

import (
	"time"

	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func RecordIndex(label string) testlib.Action {
	return func(e *types.Event, ctx *testlib.Context) (msgs []*types.Message) {
		msg, ok := util.GetMessageFromEvent(e, ctx)
		if !ok {
			return
		}
		ctx.Vars.Set(label, int(msg.Index))
		return
	}
}

func IsSameIndex(label string) testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
		msg, ok := util.GetMessageFromEvent(e, c)
		if !ok {
			return false
		}
		index, ok := c.Vars.GetInt(label)
		c.Logger().With(log.LogParams{
			"ok":       ok,
			"curIndex": index,
			"msgIndex": msg.Index,
		}).Info("checking index")
		return ok && int(msg.Index) == index
	}
}

// Drop append message and expect another one from the leader for the same index

func DropAppend() *testlib.TestCase {
	sm := testlib.NewStateMachine()
	init := sm.Builder()
	init.On(
		testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgApp)),
		"AppendObserved",
	).On(
		testlib.IsMessageSend().
			And(util.IsMessageType(raftpb.MsgApp)).
			And(util.IsReceiverSameAs("r")).
			And(IsSameIndex("appIndex")),
		testlib.SuccessStateLabel,
	)

	filters := testlib.NewFilterSet()
	filters.AddFilter(
		testlib.If(
			testlib.IsMessageSend().
				And(util.IsMessageType(raftpb.MsgApp)),
		).Then(
			testlib.OnceAction(RecordIndex("appIndex")),
			testlib.OnceAction(util.RecordMessageReceiver("r")),
			testlib.OnceAction(testlib.DropMessage()),
		),
	)

	testcase := testlib.NewTestCase(
		"DropAppend",
		1*time.Minute,
		sm,
		filters,
	)
	return testcase
}
