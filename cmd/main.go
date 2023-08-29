package cmd

import "github.com/spf13/cobra"

var iterations int

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "raft-testing",
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.PersistentFlags().IntVarP(&iterations, "iterations", "i", 1000, "number of strategy iterations to run")
	cmd.AddCommand(PCTStrategyCommand())
	cmd.AddCommand(PCTTestStrategyCommand())
	return cmd
}
