package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/unittest"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
)

var testStrat = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

		r := newRecords()
		var strategy strategies.Strategy = unittest.NewTestCaseStrategy(LivenessBugOne())

		property := sm.NewStateMachine()
		start := property.Builder()
		start.On(
			util.IsLeader(types.ReplicaID("4")),
			"FourLeader",
		).On(util.IsStateLeader(), sm.SuccessStateLabel)

		strategy = strategies.NewStrategyWithProperty(strategy, property)

		driver := strategies.NewStrategyDriver(
			&config.Config{
				APIServerAddr: "127.0.0.1:7074",
				NumReplicas:   5,
				LogConfig: config.LogConfig{
					Format: "json",
					Path:   "/Users/srinidhin/Local/data/testing/raft/t/checker.log",
				},
			},
			&util.RaftMsgParser{},
			strategy,
			&strategies.StrategyConfig{
				Iterations:       1000,
				IterationTimeout: 15 * time.Second,
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
