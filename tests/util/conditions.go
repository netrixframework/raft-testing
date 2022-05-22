package util

import (
	"fmt"
	"strconv"

	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func IsMessageType(t raftpb.MessageType) testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
		msg, ok := GetMessageFromEvent(e, c)
		if !ok {
			return false
		}
		return msg.Type == t
	}
}

func IsAcceptingVote() testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
		msg, ok := GetMessageFromEvent(e, c)
		if !ok {
			return false
		}
		return (msg.Type == raftpb.MsgVoteResp || msg.Type == raftpb.MsgPreVoteResp) && !msg.Reject
	}
}

func IsSenderSameAs(label string) testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
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

func IsReceiverSameAs(label string) testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
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

func IsStateChange() testlib.Condition {
	return testlib.IsEventType("TermChange")
}

func IsStateLeader() testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "StateChange" {
				return false
			}
			newState, ok := eType.Params["new_state"]
			if !ok {
				return false
			}
			return newState == raft.StateLeader.String()
		default:
			return false
		}
	}
}

func IsStateCandidate() testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
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

func IsCorrectLeader() testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
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
				f := int((c.Replicas.Cap() - 1) / 2)
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

func IsTermChange() testlib.Condition {
	return testlib.IsEventType("TermChange")
}

func IsNewTerm() testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
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

func IsTerm(term int) testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
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

func IsTermGte(g int) testlib.Condition {
	return func(e *types.Event, c *testlib.Context) bool {
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
