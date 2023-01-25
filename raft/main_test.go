package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"testing"
	"time"

	netrixclient "github.com/netrixframework/go-clientlibrary"
	"github.com/netrixframework/netrix/types"
)

func TestRun(t *testing.T) {
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
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
