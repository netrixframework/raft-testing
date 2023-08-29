package util

import (
	"fmt"
	"strconv"

	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/types"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func IsMessageType(t raftpb.MessageType) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		msg, ok := GetMessageFromEvent(e, c)
		if !ok {
			return false
		}
		return msg.Type == t
	}
}

func IsAcceptingVote() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		msg, ok := GetMessageFromEvent(e, c)
		if !ok {
			return false
		}
		return (msg.Type == raftpb.MsgVoteResp || msg.Type == raftpb.MsgPreVoteResp) && !msg.Reject
	}
}

func IsSenderSameAs(label string) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		sender, ok := c.Vars.GetString(label)
		if !ok {
			return false
		}
		msg, ok := c.GetMessage(e)
		if !ok {
			return false
		}
		return sender == string(msg.From)
	}
}

func IsReceiverSameAs(label string) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		sender, ok := c.Vars.GetString(label)
		if !ok {
			return false
		}
		msg, ok := c.GetMessage(e)
		if !ok {
			return false
		}
		return sender == string(msg.To)
	}
}

func IsStateChange() sm.Condition {
	return sm.IsEventType("TermChange")
}

func IsStateLeader() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "StateChange" {
				return false
			}
			newState, ok := eType.Params["new_state"]
			if !ok {
				return false
			}
			if newState == raft.StateLeader.String() {
				c.Logger.With(log.LogParams{
					"replica": e.Replica,
					"state":   newState,
					"term":    eType.Params["term"],
				}).Debug("New leader")
				return true
			}
			return false
		default:
			return false
		}
	}
}

func IsStateFollower() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "StateChange" {
				return false
			}
			newState, ok := eType.Params["new_state"]
			if !ok {
				return false
			}
			return newState == raft.StateFollower.String()
		default:
			return false
		}
	}
}

func IsStateCandidate() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "StateChange" {
				return false
			}
			newState, ok := eType.Params["new_state"]
			if !ok {
				return false
			}
			return newState == raft.StateCandidate.String()
		default:
			return false
		}
	}
}

func IsCorrectLeader() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "StateChange" {
				return false
			}
			newState, ok := eType.Params["new_state"]
			if !ok {
				return false
			}
			if newState == raft.StateLeader.String() {
				term, ok := eType.Params["term"]
				if !ok {
					return false
				}
				votesKey := fmt.Sprintf("_votes_%s_%s", term, e.Replica)
				f := int((c.ReplicaStore.Cap() - 1) / 2)
				voteCount, ok := c.Vars.GetCounter(votesKey)
				if !ok {
					return false
				}
				return voteCount.Value() >= f
			}
			return false
		default:
			return false
		}
	}
}

func IsTermChange() sm.Condition {
	return sm.IsEventType("TermChange")
}

func IsNewTerm() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "TermChange" {
				return false
			}
			curTerm, ok := c.Vars.GetInt("_highest_term")
			if !ok {
				return false
			}

			// Get the term from the event
			termS, ok := eType.Params["term"]
			if !ok {
				return false
			}
			term, err := strconv.Atoi(termS)
			if err != nil {
				return false
			}
			return term > curTerm
		default:
			return false
		}
	}
}

func IsTerm(term int) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "TermChange" {
				return false
			}
			// Get the term from the event
			termS, ok := eType.Params["term"]
			if !ok {
				return false
			}
			termE, err := strconv.Atoi(termS)
			if err != nil {
				return false
			}
			return termE == term
		default:
			return false
		}
	}
}

func IsTermGte(g int) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "TermChange" {
				return false
			}
			// Get the term from the event
			termS, ok := eType.Params["term"]
			if !ok {
				return false
			}
			termE, err := strconv.Atoi(termS)
			if err != nil {
				return false
			}
			return termE >= g
		default:
			return false
		}
	}
}

func IsLeader(replica types.ReplicaID) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		if !e.IsGeneric() {
			return false
		}
		ty := e.Type.(*types.GenericEventType)
		if ty.T != "StateChange" || ty.Params["new_state"] != raft.StateLeader.String() {
			return false
		}
		return e.Replica == replica
	}
}

func IsSameIndex(label string) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		msg, ok := GetMessageFromEvent(e, c)
		if !ok {
			return false
		}
		index, ok := c.Vars.GetInt(label)
		return ok && int(msg.Index) == index
	}
}

func IsCommit(index int) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		if !e.IsGeneric() {
			return false
		}
		ty := e.Type.(*types.GenericEventType)
		if ty.T != "Commit" {
			return false
		}
		if ty.Params["index"] != strconv.Itoa(index) {
			return false
		}
		return true
	}
}

func MoreThanOneLeader() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		leaders, ok := c.Vars.GetInt("leaders")
		return ok && leaders > 1
	}
}
