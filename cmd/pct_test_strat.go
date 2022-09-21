package cmd

// import (
// 	"math/rand"
// 	"os"
// 	"os/signal"
// 	"syscall"
// 	"time"

// 	"github.com/netrixframework/netrix/config"
// 	"github.com/netrixframework/netrix/sm"
// 	"github.com/netrixframework/netrix/strategies"
// 	"github.com/netrixframework/netrix/strategies/pct"
// 	"github.com/netrixframework/netrix/testlib"
// 	"github.com/netrixframework/raft-testing/tests/util"
// 	"github.com/spf13/cobra"
// )

// var pctTestStrat = &cobra.Command{
// 	Use: "pct-test",
// 	RunE: func(cmd *cobra.Command, args []string) error {
// 		termCh := make(chan os.Signal, 1)
// 		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

// 		r := newRecords()

// 		property := sm.NewStateMachine()
// 		builder := property.Builder()
// 		builder.On(IsCommit(1), sm.SuccessStateLabel)

// 		testcase := testlib.NewTestCase("NoFilters", 10*time.Minute, property, testlib.NewFilterSet())

// 		var strategy strategies.Strategy = pct.NewPCTStrategyWithTest()

// 		driver := strategies.NewStrategyDriver(
// 			&config.Config{
// 				APIServerAddr: "127.0.0.1:7074",
// 				NumReplicas:   3,
// 				LogConfig: config.LogConfig{
// 					Format: "json",
// 					Path:   "/Users/srinidhin/Local/data/testing/raft/t/checker.log",
// 				},
// 			},
// 			&util.RaftMsgParser{},
// 			strategy,
// 			&strategies.StrategyConfig{
// 				Iterations:       10,
// 				IterationTimeout: 10 * time.Second,
// 				SetupFunc:        r.setupFunc,
// 				StepFunc:         r.stepFunc,
// 				FinalizeFunc:     r.finalize,
// 			},
// 		)

// 		go func() {
// 			<-termCh
// 			driver.Stop()
// 		}()
// 		return driver.Start()
// 	},
// }
