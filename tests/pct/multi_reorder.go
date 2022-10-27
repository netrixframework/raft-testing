package pct

import (
	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// Expecting the vote response and append entries response to arrive later than the heartbeat
func MultiReorderProperty() *sm.StateMachine {
	property := sm.NewStateMachine()

	start := property.Builder()

	start.On(
		sm.IsMessageReceive().And(util.IsMessageType(raftpb.MsgVoteResp).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		"VoteRespReceived",
	)
	start.On(
		sm.IsMessageReceive().And(util.IsMessageType(raftpb.MsgAppResp).And(sm.IsMessageFrom(types.ReplicaID("4")))),
		"AppRespReceived",
	)

	start.On(
		sm.IsMessageSend().And(util.IsMessageType(raftpb.MsgHeartbeat).And(sm.IsMessageTo(types.ReplicaID("4")))),
		"HeartbeatSent",
	).MarkSuccess()
	return property
}
