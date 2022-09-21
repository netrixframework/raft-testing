package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/timeout"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
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
			Rate: 1.5,
			Src:  rand.NewSource(uint64(time.Now().UnixMilli())),
		},
	}
}

func (e *ExponentialDistribution) Rand() int {
	return int(e.dist.Rand())
}

var timeoutStrat = &cobra.Command{
	Use: "timeout",
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
			// PendingEventThreshold: 5,
			RecordFilePath: "/Users/srinidhin/Local/data/testing/raft/7",
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
