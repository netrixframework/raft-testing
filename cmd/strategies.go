package cmd

import (
	"sync"
	"time"

	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/types"
	raft "github.com/netrixframework/raft-testing/raft/protocol"
	"github.com/netrixframework/raft-testing/raft/protocol/raftpb"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
)

type records struct {
	totalStartTime time.Time
	duration       map[int][]time.Duration
	curStartTime   time.Time
	lock           *sync.Mutex
	timeSet        bool
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
	if r.totalStartTime.IsZero() {
		r.totalStartTime = time.Now()
	}
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
	totalRunTime := time.Since(r.totalStartTime)
	r.lock.Unlock()
	if count != 0 {
		avg := time.Duration(sum / count)
		ctx.Logger.With(log.LogParams{
			"completed_runs":    iterations,
			"average_time":      avg.String(),
			"elections_per_run": count / iterations,
			"total_run_time":    totalRunTime.String(),
		}).Info("Metrics")
	}
}

func StrategyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "strat",
	}
	cmd.AddCommand(timeoutStrat)
	cmd.AddCommand(pctStrat)
	cmd.AddCommand(pctTestStrat)
	cmd.AddCommand(testStrat)
	return cmd
}
