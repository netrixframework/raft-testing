package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	netrixclient "github.com/netrixframework/go-clientlibrary"
	"github.com/netrixframework/netrix/types"
)

var configPath *string = flag.String("conf", "", "Config file path")

type Config struct {
	Peers      string `json:"peers"`
	APIPort    string `json:"api_port"`
	ID         int    `json:"id"`
	NetrixAddr string `json:"netrix_addr"`
	ClientAddr string `json:"client_addr"`
}

func ConfigFromJson(s []byte) *Config {
	c := &Config{}
	if err := json.Unmarshal(s, c); err != nil {
		panic("Could not decode config")
	}
	return c
}

func Run(ctx context.Context, config *Config) {

	kvApp := newKVApp()

	node, err := newNode(&nodeConfig{
		ID:         config.ID,
		Peers:      strings.Split(config.Peers, ","),
		TickTime:   100 * time.Millisecond,
		StorageDir: fmt.Sprintf("raftexample-%d", config.ID),
		KVApp:      kvApp,
		TransportConfig: &netrixclient.Config{
			ReplicaID:        types.ReplicaID(strconv.Itoa(config.ID)),
			NetrixAddr:       config.NetrixAddr,
			ClientServerAddr: config.ClientAddr,
			Info:             map[string]interface{}{},
		},
	})
	if err != nil {
		log.Fatalf("failed to create node: %s", err)
	}

	node.Start()
	<-ctx.Done()
	node.Stop()

	// the key-value http handler will propose updates to raft
	serveHttpKVAPI(ctx, config.APIPort, kvApp, node)
}

func openConfFile(path string) []byte {
	s, err := ioutil.ReadFile(path)
	if err != nil {
		panic("Could not open config file")
	}
	return s
}

func main() {
	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		oscall := <-termCh
		log.Printf("Received syscall: %#v", oscall)
		cancel()
	}()

	flag.Parse()
	config := ConfigFromJson(openConfFile(*configPath))
	Run(ctx, config)
}
