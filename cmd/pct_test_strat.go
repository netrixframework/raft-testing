package cmd

import (
	"errors"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/pct"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
)

func PCTTestStrategyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "pct-test [test]",
		Long: "Run PCT with a test case to guide the exploration and measure outcomes",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			termCh := make(chan os.Signal, 1)
			signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

			testCase, property := GetTest(args[0])
			if testCase == nil || property == nil {
				return errors.New("invalid test")
			}

			r := newRecords()

			var strategy strategies.Strategy = pct.NewPCTStrategyWithTestCase(&pct.PCTStrategyConfig{
				RandSrc:        rand.NewSource(time.Now().UnixMilli()),
				MaxEvents:      100,
				Depth:          6,
				RecordFilePath: "results",
			}, testCase)

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
	return cmd
}

// filters := testlib.NewFilterSet()
// filters.AddFilter(
// 	testlib.If(util.IsMessageType(raftpb.MsgVote).Or(util.IsMessageType(raftpb.MsgVoteResp)).And(
// 		testlib.IsMessageAcrossPartition())).Then(testlib.DropMessage()),
// )

// testCase := testlib.NewTestCase("Partition", 10*time.Minute, sm.NewStateMachine(), filters)
// testCase.SetupFunc(func(ctx *testlib.Context) error {
// 	ctx.CreatePartition([]int{2, 3}, []string{"one", "two"})
// 	return nil
// })

// property := sm.NewStateMachine()
// start := property.Builder()
// start.On(IsCommit(6), sm.SuccessStateLabel)

// start.On(
// 	util.IsLeader(types.ReplicaID("4")),
// 	"FourLeader",
// ).On(util.IsStateLeader(), sm.SuccessStateLabel)

// start.On(
// 	sm.ConditionWithAction(util.IsStateLeader(), CountLeaderChanges()),
// 	sm.StartStateLabel,
// )
// start.On(
// 	sm.Count("leaderCount").Gt(4),
// 	sm.FailStateLabel,
// )
// start.MarkSuccess()
