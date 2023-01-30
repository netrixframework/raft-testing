package cmd

import (
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/pct"
	"github.com/netrixframework/netrix/types"
	raft "github.com/netrixframework/raft-testing/raft/protocol"
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

func IsCommit(index int) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		if !e.IsGeneric() {
			return false
		}
		ty := e.Type.(*types.GenericEventType)
		if ty.T != "Commit" {
			return false
		}
		if ty.Params["index"] != strconv.Itoa(index) {
			return false
		}
		return true
	}
}

func CountTermLeader() sm.Action {
	return func(e *types.Event, ctx *sm.Context) {
		switch eType := e.Type.(type) {
		case *types.GenericEventType:
			if eType.T != "StateChange" {
				return
			}
			newState, ok := eType.Params["new_state"]
			if !ok {
				return
			}
			if newState == raft.StateLeader.String() {
				key := "leaders"
				if ctx.Vars.Exists(key) {
					cur, _ := ctx.Vars.GetInt(key)
					ctx.Vars.Set(key, cur+1)
				} else {
					ctx.Vars.Set(key, 1)
				}
			}
		default:
		}
	}
}

func MoreThanOneLeader() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		leaders, ok := c.Vars.GetInt("leaders")
		return ok && leaders > 1
	}
}

var pctStrat = &cobra.Command{
	Use: "pct",
	RunE: func(cmd *cobra.Command, args []string) error {
		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

		// r := newRecords()

		rInt := newRaftInterpreter("/local/snagendra/data/testing/raft/t/states.jsonl")

		var strategy strategies.Strategy = pct.NewPCTStrategy(&pct.PCTStrategyConfig{
			RandSrc:        rand.NewSource(time.Now().UnixMilli()),
			MaxEvents:      1000,
			Depth:          6,
			RecordFilePath: "/local/snagendra/data/testing/raft/t",
		})

		// strategy = strategies.NewStrategyWithProperty(strategy, pctTest.MultiReorderProperty())

		driver := strategies.NewStrategyDriver(
			&config.Config{
				APIServerAddr: "127.0.0.1:7074",
				NumReplicas:   5,
				LogConfig: config.LogConfig{
					Format: "json",
					Path:   "/local/snagendra/data/testing/raft/t/checker.log",
				},
			},
			&util.RaftMsgParser{},
			strategy,
			&strategies.StrategyConfig{
				Iterations:       1000,
				IterationTimeout: 4 * time.Second,
				SetupFunc: func(ctx *strategies.Context) {
					rInt.Reset()
				},
				StepFunc: func(e *types.Event, ctx *strategies.Context) {
					rInt.Update(e, ctx)
				},
				FinalizeFunc: func(ctx *strategies.Context) {
					ctx.Logger.With(log.LogParams{
						"unique_states": rInt.CoveredStates(),
					}).Info("Covered states")
					rInt.RecordCoverage()
				},
			},
		)

		go func() {
			<-termCh
			driver.Stop()
		}()
		return driver.Start()
	},
}

// property := sm.NewStateMachine()
// start := property.Builder()
// // start.On(IsCommit(6), sm.SuccessStateLabel)
// start.On(
// 	sm.ConditionWithAction(util.IsStateLeader(), CountTermLeader()),
// 	sm.StartStateLabel,
// )
// start.On(MoreThanOneLeader(), sm.SuccessStateLabel)
