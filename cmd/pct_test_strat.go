package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/pct"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"github.com/netrixframework/raft-testing/raft/protocol/raftpb"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
)

func IsNewCommit() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		if !e.IsGeneric() {
			return false
		}
		ty := e.Type.(*types.GenericEventType)
		if ty.T != "Commit" {
			return false
		}
		key := fmt.Sprintf("commit_%s", ty.Params["index"])
		return !c.Vars.Exists(key)
	}
}

func RecordCommit() sm.Action {
	return func(e *types.Event, ctx *sm.Context) {
		if !e.IsGeneric() {
			return
		}
		ty := e.Type.(*types.GenericEventType)
		if ty.T != "Commit" {
			return
		}

		key := fmt.Sprintf("commit_%s", ty.Params["index"])
		if !ctx.Vars.Exists(key) {
			ctx.Vars.Set(key, ty.Params["entry"])
		}
	}
}

func IsDifferentCommit() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		if !e.IsGeneric() {
			return false
		}
		ty := e.Type.(*types.GenericEventType)
		if ty.T != "Commit" {
			return false
		}
		key := fmt.Sprintf("commit_%s", ty.Params["index"])
		cur, exists := c.Vars.GetString(key)
		if !exists {
			return false
		}
		if cur != ty.Params["entry"] {
			c.Logger.With(log.LogParams{
				"current": cur,
				"new":     ty.Params["entry"],
				"index":   ty.Params["index"],
			}).Info("observed different commits for same index")
			return true
		}
		return false
	}
}

func IsConfChangeApp() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		if !e.IsMessageSend() {
			return false
		}
		message, ok := c.GetMessage(e)
		if !ok {
			return false
		}
		raftMessage := message.ParsedMessage.(*util.RaftMsgWrapper)
		if raftMessage.Type != raftpb.MsgApp {
			return false
		}
		confChange := false
		for _, entry := range raftMessage.Entries {
			if entry.Type != raftpb.EntryNormal {
				confChange = true
			}
		}
		return confChange
	}
}

func DeleteNode(node, to types.ReplicaID) testlib.Action {
	return func(e *types.Event, ctx *testlib.Context) (messages []*types.Message) {
		replica, ok := ctx.ReplicaStore.Get(to)
		if !ok {
			return
		}
		addr, ok := replica.Info["http_api_addr"].(string)
		if !ok {
			return
		}
		deleteNode(addr, string(node))
		return
	}
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
			if replica.ID == types.ReplicaID("1") {
				deleteNode(addrS, "4")
			} else if replica.ID == types.ReplicaID("2") {
				deleteNode(addrS, "3")
			}
		}
	}
}

func ConfChangeTest() *testlib.TestCase {
	filters := testlib.NewFilterSet()

	// First bug where we just create a network partition
	// filters.AddFilter(
	// 	testlib.If(testlib.IsMessageAcrossPartition()).Then(testlib.DropMessage()),
	// )

	// Second test to force a safety violation

	testStateMachine := sm.NewStateMachine()
	oneLeader := testStateMachine.Builder().On(util.IsLeader(types.ReplicaID("1")), "OneLeader")

	removed3 := oneLeader.On(util.IsCommitFor(util.Remove(types.ReplicaID("3"))), "Removed3")
	removed3.MarkSuccess()

	filters.AddFilter(
		testlib.If(
			testStateMachine.InState(sm.StartStateLabel).And(
				util.IsMessageType(raftpb.MsgVote).And(
					sm.IsMessageFrom(types.ReplicaID("1")).Not(),
				).Or(
					util.IsMessageType(raftpb.MsgApp).And(sm.IsMessageFrom(types.ReplicaID("1"))),
				),
			),
		).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(
			testStateMachine.InState("OneLeader").And(
				sm.IsMessageFrom(types.ReplicaID("1")).Or(sm.IsMessageTo(types.ReplicaID("1"))),
			),
		).Then(
			testlib.DropMessage(),
		),
	)
	// filters.AddFilter(
	// 	testlib.If(util.IsMessageType(raftpb.MsgApp).And(sm.IsMessageTo(types.ReplicaID("1")))).Then(testlib.DropMessage()),
	// )
	filters.AddFilter(
		testlib.If(
			testStateMachine.InState("OneLeader").And(
				util.IsMessageType(raftpb.MsgApp).And(
					sm.IsMessageTo(types.ReplicaID("3")),
				),
			),
		).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(
			testStateMachine.InState("OneLeader").And(
				util.IsMessageType(raftpb.MsgVote).And(
					sm.IsMessageFrom(types.ReplicaID("1")).Or(sm.IsMessageFrom(types.ReplicaID("3"))),
				),
			),
		).Then(testlib.DropMessage()),
	)
	filters.AddFilter(
		testlib.If(util.IsLeader(types.ReplicaID("1"))).Then(
			testlib.OnceAction("DeleteNode4", DeleteNode(types.ReplicaID("4"), types.ReplicaID("1"))),
		),
	)
	filters.AddFilter(
		testlib.If(
			util.IsLeader(types.ReplicaID("2")).Or(util.IsLeader(types.ReplicaID("4"))),
		).Then(
			testlib.OnceAction("DeleteNode3", DeleteNode(types.ReplicaID("3"), types.ReplicaID("2"))),
		),
	)
	// filters.AddFilter(
	// 	testlib.If(
	// 		testStateMachine.InState("Removed3").Not().And(
	// 			util.IsMessageType(raftpb.MsgApp).And(sm.IsMessageTo(types.ReplicaID("3"))),
	// 		),
	// 	).Then(testlib.DropMessage()),
	// )

	testCase := testlib.NewTestCase("ConfigChange", 10*time.Minute, testStateMachine, filters)
	return testCase
}

var pctTestStrat = &cobra.Command{
	Use: "pct-test",
	RunE: func(cmd *cobra.Command, args []string) error {
		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

		r := newRecords()

		testCase := ConfChangeTest()

		var strategy strategies.Strategy = pct.NewPCTStrategyWithTestCase(&pct.PCTStrategyConfig{
			RandSrc:        rand.NewSource(time.Now().UnixMilli()),
			MaxEvents:      1000,
			Depth:          6,
			RecordFilePath: "/Users/srinidhin/Local/data/testing/raft/t",
		}, testCase)

		property := sm.NewStateMachine()
		start := property.Builder()
		// start.On(
		// 	util.IsLeader(types.ReplicaID("1")),
		// 	sm.SuccessStateLabel,
		// )

		start.On(
			sm.ConditionWithAction(IsNewCommit(), RecordCommit()),
			sm.StartStateLabel,
		)
		start.On(
			IsDifferentCommit(),
			sm.SuccessStateLabel,
		)

		strategy = strategies.NewStrategyWithProperty(strategy, property)

		driver := strategies.NewStrategyDriver(
			&config.Config{
				APIServerAddr: "127.0.0.1:7074",
				NumReplicas:   4,
				LogConfig: config.LogConfig{
					Format: "json",
					Path:   "/Users/srinidhin/Local/data/testing/raft/t/checker.log",
				},
			},
			&util.RaftMsgParser{},
			strategy,
			&strategies.StrategyConfig{
				Iterations:       50,
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
