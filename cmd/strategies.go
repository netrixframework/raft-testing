package cmd

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/timeout"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
)

type ParetoDistribution struct {
	dist *distuv.Pareto
}

func NewParetoDistribution() *ParetoDistribution {
	return &ParetoDistribution{
		dist: &distuv.Pareto{
			Xm:    0.235,
			Alpha: 10,
			Src:   rand.NewSource(uint64(time.Now().UnixMilli())),
		},
	}
}

func (p *ParetoDistribution) Rand() int {
	return int(p.dist.Rand())
}

type ExponentialDistribution struct {
	dist *distuv.Exponential
}

func NewExponentialDistribution() *ExponentialDistribution {
	return &ExponentialDistribution{
		dist: &distuv.Exponential{
			Rate: 1,
			Src:  rand.NewSource(uint64(time.Now().UnixMilli())),
		},
	}
}

func (e *ExponentialDistribution) Rand() int {
	return int(e.dist.Rand())
}

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
		message, ok := ctx.Messages.Get(eType.MessageID)
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
		}).Info("Metrics")
	}
}

var strategyCmd = &cobra.Command{
	Use: "strat",
	RunE: func(cmd *cobra.Command, args []string) error {
		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

		r := newRecords()

		strategy, err := timeout.NewTimeoutStrategy(&timeout.TimeoutStrategyConfig{
			Nondeterministic:  true,
			SpuriousCheck:     true,
			ClockDrift:        5,
			MaxMessageDelay:   100 * time.Millisecond,
			DelayDistribution: NewExponentialDistribution(),
			MessageBias:       0.8,
			RecordFilePath:    "/Users/srinidhin/Local/data/testing/raft/7",
		})
		if err != nil {
			return err
		}

		driver := strategies.NewStrategyDriver(
			&config.Config{
				APIServerAddr: "172.23.37.208:7074",
				NumReplicas:   3,
				LogConfig: config.LogConfig{
					Format: "json",
					Path:   "/Users/srinidhin/Local/data/testing/raft/7/checker.log",
				},
			},
			&util.RaftMsgParser{},
			strategy,
			&strategies.StrategyConfig{
				Iterations:       100,
				IterationTimeout: 10 * time.Second,
				SetupFunc:        r.setupFunc,
				StepFunc:         r.stepFunc,
				FinalizeFunc:     r.finalize,
			},
		)

		go func() {
			<-termCh
			driver.Stop()
		}()
		return driver.Start()
	},
}
