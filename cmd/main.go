package cmd

import "github.com/spf13/cobra"

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "raft-testing",
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.AddCommand(strategyCmd)
	cmd.AddCommand(unittestCmd)
	return cmd
}
