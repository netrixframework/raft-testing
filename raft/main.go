package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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

func openConfFile(path string) []byte {
	s, err := ioutil.ReadFile(path)
	if err != nil {
		panic("Could not open config file")
	}
	return s
}

func main() {

	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, os.Interrupt)

	flag.Parse()
	config := ConfigFromJson(openConfFile(*configPath))
	kvApp := newKVApp()
	node, err := newNode(&nodeConfig{
		ID:         config.ID,
		Peers:      strings.Split(config.Peers, ","),
		TickTime:   100 * time.Millisecond,
		StorageDir: fmt.Sprintf("build/storage/raftexample-%d", config.ID),
		KVApp:      kvApp,
		LogPath:    fmt.Sprintf("build/logs/raftexample-%d", config.ID),
		TransportConfig: &netrixclient.Config{
			ReplicaID:        types.ReplicaID(strconv.Itoa(config.ID)),
			NetrixAddr:       config.NetrixAddr,
			ClientServerAddr: config.ClientAddr,
			Info: map[string]interface{}{
				"http_api_addr": "127.0.0.1:" + config.APIPort,
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to create node: %s", err)
	}
	node.ResetStorage()

	// the key-value http handler will propose updates to raft
	api := newHTTPKVAPI(kvApp, node)
	api.Start()
	srv := http.Server{
		Addr:    "127.0.0.1:" + config.APIPort,
		Handler: api,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	node.Start()

	oscall := <-termCh
	log.Printf("Received syscall: %#v", oscall)
	node.Stop()
	api.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
}
