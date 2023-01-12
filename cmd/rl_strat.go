package cmd

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/rl"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
)

func rlSetup() func(*strategies.Context) {
	return func(ctx *strategies.Context) {
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

func getPolicy(name string) (rl.Policy, error) {
	switch name {
	case "ucbzero":
		return rl.NewUCBZeroPolicy(&rl.UCBZeroPolicyConfig{
			Horizon:     1000,
			StateSpace:  10000,
			Iterations:  10,
			ActionSpace: 1000,

			Probability: 0.2,
			C:           1,
		}), nil
	case "ucbzerogreedy":
		return rl.NewUCBZeroEGreedyPolicy(&rl.UCBZeroEGreedyPolicyConfig{
			UCBZeroPolicyConfig: &rl.UCBZeroPolicyConfig{
				Horizon:     1000,
				StateSpace:  10000,
				Iterations:  10,
				ActionSpace: 1000,

				Probability: 0.2,
				C:           1,
			},
			Epsilon: 0.1,
		})
	case "softmax":
		return rl.NewNegativeRewardPolicy(0.3, 0.7), nil
	default:
		return nil, errors.New("invalid policy name")
	}
}

var rlStratCmd = &cobra.Command{
	Use:   "rl [policy]",
	Short: "rl [policy] - policy can be one of ucbzero,ucbzerogreedy or softmax",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

		policy, err := getPolicy(args[0])
		if err != nil {
			return err
		}

		interpreter := newRaftInterpreter("/Users/srinidhin/Local/data/testing/raft/t/states.jsonl")
		strategy := rl.NewRLStrategy(&rl.RLStrategyConfig{
			Interpreter:       interpreter,
			AgentTickDuration: 20 * time.Millisecond,
			Policy:            policy,
		})

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
				SetupFunc:        rlSetup(),
				FinalizeFunc: func(ctx *strategies.Context) {
					ctx.Logger.With(log.LogParams{
						"unique_states": interpreter.CoveredStates(),
					}).Info("Covered states")
					interpreter.RecordCoverage()
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