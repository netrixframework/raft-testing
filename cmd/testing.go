package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/netrixframework/netrix/config"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/raft-testing/tests/appends"
	"github.com/netrixframework/raft-testing/tests/util"
	"github.com/spf13/cobra"
)

var unittestCmd = &cobra.Command{
	Use: "unit",
	RunE: func(cmd *cobra.Command, args []string) error {

		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

		server, err := testlib.NewTestingServer(
			&config.Config{
				APIServerAddr: "172.28.47.54:7074",
				NumReplicas:   3,
				LogConfig: config.LogConfig{
					Format: "json",
					Path:   "/tmp/raft/log/checker.log",
				},
			},
			&util.RaftMsgParser{},
			[]*testlib.TestCase{
				// tests.AllowAllTest(),
				// normal.VoteResponse(),
				// normal.HeartbeatResponse(),
				// normal.ExpectLeader(),
				// normal.ExpectHeartbeat(),
				// normal.ExpectAppend(),
				// voting.DropVotes(),
				// voting.DropVotesNewTerm(),
				// voting.DropHeartbeat(),
				// voting.Partition(),
				appends.DropAppend(),
			},
		)
		if err != nil {
			fmt.Printf("Failed to start server: %s\n", err.Error())
			os.Exit(1)
		}
		go func() {
			<-termCh
			server.Stop()
		}()
		server.Start()
		return nil
	},
}
