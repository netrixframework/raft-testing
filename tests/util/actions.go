package util

import (
	"fmt"
	"strconv"

	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func SetKeyValueAction(key, value string) testlib.Action {
	return func(e *types.Event, ctx *testlib.Context) (msgs []*types.Message) {
		replica, ok := ctx.ReplicaStore.GetRandom()
		if !ok {
			return
		}
		SetKeyValue(replica, key, value)
		return
	}
}

func RecordMessageSender(label string) testlib.Action {
	return func(e *types.Event, ctx *testlib.Context) (msgs []*types.Message) {
		msg, ok := ctx.GetMessage(e)
		if !ok {
			return
		}
		ctx.Vars.Set(label, string(msg.From))
		return
	}
}

func RecordMessageReceiver(label string) testlib.Action {
	return func(e *types.Event, ctx *testlib.Context) (msgs []*types.Message) {
		msg, ok := ctx.GetMessage(e)
		if !ok {
			return
		}
		ctx.Logger.With(log.LogParams{
			"to": string(msg.To),
		}).Debug("recording receiver")
		ctx.Vars.Set(label, string(msg.To))
		return
	}
}

func CountVotes() testlib.FilterFunc {
	return func(e *types.Event, ctx *testlib.Context) (msgs []*types.Message, ok bool) {
		msg, ok := GetMessageFromEvent(e, ctx.Context)
		if !ok {
			return msgs, false
		}
		if msg.Type != raftpb.MsgVoteResp || msg.Reject {
			return msgs, false
		}
		key := fmt.Sprintf("_votes_%d_%d", msg.Term, msg.To)
		if !ctx.Vars.Exists(key) {
			ctx.Vars.SetCounter(key)
		}
		counter, _ := ctx.Vars.GetCounter(key)
		counter.Incr()
		return msgs, false
	}
}

func CountTerm() testlib.FilterFunc {
	return func(e *types.Event, ctx *testlib.Context) (msgs []*types.Message, ok bool) {
		msg, ok := GetMessageFromEvent(e, ctx.Context)
		if !ok {
			return msgs, false
		}
		key := "_highest_term"
		if !ctx.Vars.Exists(key) {
			ctx.Vars.Set(key, 0)
		}
		curTerm, _ := ctx.Vars.GetInt(key)
		if msg.Term > uint64(curTerm) {
			ctx.Vars.Set(key, int(msg.Term))
		}
		return msgs, false
	}
}

func TrackLeader() testlib.FilterFunc {
	return func(e *types.Event, ctx *testlib.Context) (msgs []*types.Message, ok bool) {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "StateChange" {
				return msgs, false
			}
			newState, ok := eType.Params["new_state"]
			if !ok {
				return msgs, false
			}
			if newState != raft.StateLeader.String() {
				return msgs, false
			}
			term, _ := strconv.Atoi(eType.Params["term"])
			curTerm, ok := ctx.Vars.GetInt("_highest_term")
			if !ok || curTerm <= term {
				key := fmt.Sprintf("_leader_%s", eType.Params["term"])
				ctx.Vars.Set(key, e.Replica)
				ctx.Vars.Set("_highest_term", term)
			}
			return msgs, false
		default:
			return msgs, false
		}
	}
}

func RecordTerm(as string) testlib.Action {
	return func(e *types.Event, ctx *testlib.Context) (messages []*types.Message) {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T == "StateChange" {
				ctx.Vars.Set(as, eType.Params["term"])
			}
		case *types.MessageSendEventType:
			message, ok := GetMessageFromEvent(e, ctx.Context)
			if !ok {
				return
			}
			ctx.Vars.Set(as, strconv.FormatUint(message.Term, 10))
			return
		}
		return
	}
}

func CountLeaderChanges() sm.Action {
	return func(e *types.Event, ctx *sm.Context) {
		if !e.IsGeneric() {
			return
		}
		ty := e.Type.(*types.GenericEventType)
		if ty.T != "StateChange" || ty.Params["new_state"] != raft.StateLeader.String() {
			return
		}
		if curLeader, ok := ctx.Vars.GetString("leader"); ok && curLeader != string(e.Replica) {
			ctx.Vars.Set("leader", string(e.Replica))
			if !ctx.Vars.Exists("leaderCount") {
				ctx.Vars.SetCounter("leaderCount")
			}
			counter, _ := ctx.Vars.GetCounter("leaderCount")
			counter.Incr()
		}
	}
}

func RecordIndex(label string) testlib.Action {
	return func(e *types.Event, ctx *testlib.Context) (msgs []*types.Message) {
		msg, ok := GetMessageFromEvent(e, ctx.Context)
		if !ok {
			return
		}
		ctx.Vars.Set(label, int(msg.Index))
		return
	}
}
