package main

import (
	"github.com/netrixframework/raft-testing/cmd"
)

func main() {
	c := cmd.RootCmd()
	c.Execute()
}
