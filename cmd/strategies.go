package cmd

import (
	"sync"
	"time"

	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests"
	"github.com/netrixframework/raft-testing/tests/util"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type records struct {
	duration     map[int][]time.Duration
	curStartTime time.Time
	lock         *sync.Mutex
	timeSet      bool
}

func newRecords() *records {
	return &records{
		duration: make(map[int][]time.Duration),
		lock:     new(sync.Mutex),
	}
}

func (r *records) setupFunc(*strategies.Context) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.curStartTime = time.Now()
	r.timeSet = true
}

func (r *records) stepFunc(e *types.Event, ctx *strategies.Context) {
	switch eType := e.Type.(type) {
	case *types.MessageSendEventType:
		message, ok := ctx.MessagePool.Get(eType.MessageID)
		if ok {
			rMsg, ok := message.ParsedMessage.(*util.RaftMsgWrapper)
			if ok {
				r.lock.Lock()
				timeSet := r.timeSet
				r.lock.Unlock()
				if rMsg.Type == raftpb.MsgVote && !timeSet {
					r.lock.Lock()
					r.curStartTime = time.Now()
					r.timeSet = true
					r.lock.Unlock()
				}
			}
		}
	case *types.GenericEventType:
		if eType.T == "StateChange" {
			newState, ok := eType.Params["new_state"]
			var dur time.Duration
			if ok && newState == raft.StateLeader.String() {
				r.lock.Lock()
				_, ok = r.duration[ctx.CurIteration()]
				if !ok {
					r.duration[ctx.CurIteration()] = make([]time.Duration, 0)
				}
				dur = time.Since(r.curStartTime)
				r.duration[ctx.CurIteration()] = append(r.duration[ctx.CurIteration()], dur)
				r.timeSet = false
				r.lock.Unlock()
			}
		}
	}
}

func (r *records) finalize(ctx *strategies.Context) {
	sum := 0
	count := 0
	r.lock.Lock()
	for _, dur := range r.duration {
		for _, d := range dur {
			sum = sum + int(d)
			count = count + 1
		}
	}
	iterations := len(r.duration)
	r.lock.Unlock()
	if count != 0 {
		avg := time.Duration(sum / count)
		ctx.Logger.With(log.LogParams{
			"completed_runs":    iterations,
			"average_time":      avg.String(),
			"elections_per_run": count / iterations,
		}).Debug("Metrics")
	}
}

func GetTest(test string) (*testlib.TestCase, *sm.StateMachine) {
	switch test {
	case "Liveness":
		return tests.LivenessTest(), tests.LivenessProperty()
	case "LivenessNoCQ":
		return tests.LivenessNoCQTest(), tests.LivenessNoCQProperty()
	case "NoLiveness":
		return tests.NoLivenessTest(), tests.NoLivenessProperty()
	case "ConfChangeBug":
		return tests.ConfChangeBugTest(), tests.ConfChangeBugProperty()
	case "DropHeartbeat":
		return tests.DropHeartbeatTest(), tests.DropHeartbeatProperty()
	case "DropVotes":
		return tests.DropVotesTest(), tests.DropVotesProperty()
	case "DropFVotes":
		return tests.DropFVotesTest(), tests.DropFVotesProperty()
	case "DropAppend":
		return tests.DropAppendTest(), tests.DropAppendProperty()
	case "ReVote":
		return tests.ReVoteTest(), tests.ReVoteProperty()
	case "ManyReVote":
		return tests.ManyReVoteTest(), tests.ManyReVoteProperty()
	case "MultiReVote":
		return tests.MultiReVoteTest(), tests.MultiReVoteProperty()
	}
	return nil, nil
}
