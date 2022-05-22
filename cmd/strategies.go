package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/timeout"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/raft/v3"
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
			Rate: 1.66,
			Src:  rand.NewSource(uint64(time.Now().UnixMilli())),
		},
	}
}

func (e *ExponentialDistribution) Rand() int {
	return int(e.dist.Rand())
}

type records struct {
	duration     map[int]time.Duration
	curStartTime time.Time
	lock         *sync.Mutex
}

func newRecords() *records {
	return &records{
		duration: make(map[int]time.Duration),
		lock:     new(sync.Mutex),
	}
}

func (r *records) setupFunc(*strategies.Context) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.curStartTime = time.Now()

	return nil
}

func (r *records) stepFunc(e *types.Event, ctx *strategies.Context) {
	switch eType := e.Type.(type) {
	case *types.GenericEventType:
		if eType.T == "StateChange" {
			newState, ok := eType.Params["new_state"]
			if ok && newState == raft.StateLeader.String() {
				r.lock.Lock()
				r.duration[ctx.CurIteration()] = time.Since(r.curStartTime)
				r.lock.Unlock()
			}
		}
	}
}

func (r *records) finalize() {
	sum := 0
	r.lock.Lock()
	for _, dur := range r.duration {
		sum = sum + int(dur)
	}
	count := len(r.duration)
	r.lock.Unlock()
	avg := time.Duration(sum / count)
	fmt.Printf("\nAverage time of run: %s\n", avg.String())
}

var strategyCmd = &cobra.Command{
	Use: "strat",
	RunE: func(cmd *cobra.Command, args []string) error {
		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

		r := newRecords()

		strategy, err := timeout.NewTimeoutStrategy(&timeout.TimeoutStrategyConfig{
			Nondeterministic: true,
			ClockDrift:       5,
			MaxMessageDelay:  100 * time.Millisecond,
		})
		if err != nil {
			return err
		}

		driver := strategies.NewStrategyDriver(
			&config.Config{
				APIServerAddr: "10.0.0.8:7074",
				NumReplicas:   3,
				LogConfig: config.LogConfig{
					Format: "json",
					Path:   "/tmp/raft/log/checker.log",
				},
				StrategyConfig: config.StrategyConfig{
					Iterations:       30,
					IterationTimeout: 10 * time.Second,
				},
			},
			&util.RaftMsgParser{},
			strategy,
			r.setupFunc,
			r.stepFunc,
		)

		go func() {
			<-termCh
			r.finalize()
			driver.Stop()
		}()
		return driver.Start()
	},
}
