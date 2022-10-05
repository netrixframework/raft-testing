package cmd

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/sm"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/pct"
	"github.com/netrixframework/netrix/types"
	raft "github.com/netrixframework/raft-testing/raft/protocol"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
)

func setKeyValue(apiAddr, key, value string) error {
	req, err := http.NewRequest(http.MethodPut, "http://"+apiAddr+"/"+key, strings.NewReader(value))
	if err != nil {
		return err
	}
	client := &http.Client{}
	_, err = client.Do(req)
	return err
}

func deleteNode(apiAddr, node string) error {
	req, err := http.NewRequest(http.MethodDelete, "http://"+apiAddr+"/"+node, nil)
	if err != nil {
		return err
	}
	client := &http.Client{}
	_, err = client.Do(req)
	return err
}

func IsCommitAt(index int) sm.Condition {
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

func IsConfigChange() sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		if !e.IsGeneric() {
			return false
		}
		ty := e.Type.(*types.GenericEventType)
		return ty.T == "ConfigChange"
	}
}

func IsConfigChangeTo(voters []int) sm.Condition {
	return func(e *types.Event, c *sm.Context) bool {
		if !e.IsGeneric() {
			return false
		}
		ty := e.Type.(*types.GenericEventType)
		if ty.T != "ConfigChange" {
			return false
		}
		// c.Logger.With(log.LogParams{
		// 	"change":   ty.Params["voters"],
		// 	"required": fmt.Sprintf("%v", voters),
		// }).Info("Comparing config change")
		return ty.Params["voters"] == fmt.Sprintf("%v", voters)
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

		r := newRecords()

		var strategy strategies.Strategy = pct.NewPCTStrategy(&pct.PCTStrategyConfig{
			RandSrc:        rand.NewSource(time.Now().UnixMilli()),
			MaxEvents:      1000,
			Depth:          15,
			RecordFilePath: "/Users/srinidhin/Local/data/testing/raft/t",
		})

		// property := sm.NewStateMachine()
		// start := property.Builder()
		// // start.On(IsCommit(6), sm.SuccessStateLabel)
		// start.On(
		// 	sm.ConditionWithAction(util.IsStateLeader(), CountTermLeader()),
		// 	sm.StartStateLabel,
		// )
		// start.On(MoreThanOneLeader(), sm.SuccessStateLabel)

		property := sm.NewStateMachine()
		start := property.Builder()

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
				Iterations:       100,
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
