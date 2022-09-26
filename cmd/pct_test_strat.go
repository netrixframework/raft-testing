package cmd

import (
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/pct"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

var pctTestStrat = &cobra.Command{
	Use: "pct-test",
	RunE: func(cmd *cobra.Command, args []string) error {
		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

		r := newRecords()

		filters := testlib.NewFilterSet()
		filters.AddFilter(
			testlib.If(util.IsMessageType(raftpb.MsgVote).Or(util.IsMessageType(raftpb.MsgVoteResp)).And(
				testlib.IsMessageAcrossPartition())).Then(testlib.DropMessage()),
		)

		testCase := testlib.NewTestCase("Partition", 10*time.Minute, sm.NewStateMachine(), filters)
		testCase.SetupFunc(func(ctx *testlib.Context) error {
			ctx.CreatePartition([]int{2, 3}, []string{"one", "two"})
			return nil
		})

		var strategy strategies.Strategy = pct.NewPCTStrategyWithTestCase(&pct.PCTStrategyConfig{
			RandSrc:        rand.NewSource(time.Now().UnixMilli()),
			MaxEvents:      100,
			Depth:          6,
			RecordFilePath: "/Users/srinidhin/Local/data/testing/raft/t",
		}, testCase)

		property := sm.NewStateMachine()
		start := property.Builder()
		// start.On(IsCommit(6), sm.SuccessStateLabel)
		start.On(
			sm.ConditionWithAction(util.IsStateLeader(), CountTermLeader()),
			sm.StartStateLabel,
		)
		start.On(MoreThanOneLeader(), sm.SuccessStateLabel)

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
				Iterations:       50,
				IterationTimeout: 10 * time.Second,
				SetupFunc:        pctSetupFunc(r.setupFunc),
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
