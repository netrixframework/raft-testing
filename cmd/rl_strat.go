package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/rl"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
)

var rlStratCmd = &cobra.Command{
	Use: "rl",
	RunE: func(cmd *cobra.Command, args []string) error {
		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

		strategy, err := rl.NewRLStrategy(&rl.RLStrategyConfig{
			Alpha:       0.3,
			Gamma:       0.7,
			Interpreter: rl.NewSimpleInterpreter(),
			Policy:      rl.NewSoftMaxPolicy(),
		})
		if err != nil {
			return fmt.Errorf("failed to create strategy: %s", err)
		}

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
				Iterations:       10,
				IterationTimeout: 15 * time.Second,
			},
		)

		go func() {
			<-termCh
			driver.Stop()
		}()
		return driver.Start()
	},
}
