package cmd

import (
	"errors"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/pct"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
)

func setKeyValue(ctx *strategies.Context, apiAddr, key, value string) error {
	req, err := http.NewRequest(http.MethodPut, "http://"+apiAddr+"/"+key, strings.NewReader(value))
	if err != nil {
		return err
	}
	client := &http.Client{}
	_, err = client.Do(req)
	return err
}

func pctSetupFunc(recordSetupFunc func(*strategies.Context)) func(*strategies.Context) {
	return func(ctx *strategies.Context) {
		recordSetupFunc(ctx)
		for _, replica := range ctx.ReplicaStore.Iter() {
			addrI, ok := replica.Info["http_api_addr"]
			if !ok {
				continue
			}
			addrS, ok := addrI.(string)
			if !ok {
				continue
			}
			if err := setKeyValue(ctx, addrS, "test", "test"); err == nil {
				break
			}
		}
	}
}
func PCTStrategyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "pct [test]",
		Long: "Run PCT without any guidance and check if the property is satisfied for a specific test",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			termCh := make(chan os.Signal, 1)
			signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

			_, property := GetTest(args[0])
			if property == nil {
				return errors.New("invalid test")
			}

			r := newRecords()

			var strategy strategies.Strategy = pct.NewPCTStrategy(&pct.PCTStrategyConfig{
				RandSrc:        rand.NewSource(time.Now().UnixMilli()),
				MaxEvents:      1000,
				Depth:          6,
				RecordFilePath: "results",
			})

			strategy = strategies.NewStrategyWithProperty(strategy, property)

			driver := strategies.NewStrategyDriver(
				&config.Config{
					APIServerAddr: "127.0.0.1:7074",
					NumReplicas:   5,
					LogConfig: config.LogConfig{
						Format: "json",
						Path:   "results/checker.log",
					},
				},
				&util.RaftMsgParser{},
				strategy,
				&strategies.StrategyConfig{
					Iterations:       iterations,
					IterationTimeout: 4 * time.Second,
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
	return cmd
}

// property := sm.NewStateMachine()
// start := property.Builder()
// // start.On(IsCommit(6), sm.SuccessStateLabel)
// start.On(
// 	sm.ConditionWithAction(util.IsStateLeader(), CountTermLeader()),
// 	sm.StartStateLabel,
// )
// start.On(MoreThanOneLeader(), sm.SuccessStateLabel)
