package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	netrixclient "github.com/netrixframework/go-clientlibrary"
	raft "github.com/netrixframework/raft-testing/raft/protocol"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal"
	"go.uber.org/zap"
)

type node struct {
	rn     raft.Node
	rnLock *sync.Mutex
	ID     types.ID
	peers  []raft.Peer
	config *nodeConfig
	ticker *time.Ticker

	storage     *raft.MemoryStorage
	storageLock *sync.Mutex
	wal         *wal.WAL
	state       *nodeState

	kvApp     *kvApp
	transport *netrixTransport

	logger   *zap.Logger
	doneChan chan struct{}
}

type nodeConfig struct {
	ID              int
	Peers           []string
	TickTime        time.Duration
	TransportConfig *netrixclient.Config
	KVApp           *kvApp
	StorageDir      string
	LogPath         string
}

func newNode(config *nodeConfig) (*node, error) {
	if _, err := os.Stat(config.LogPath); err == nil {
		logFilePath := path.Join(config.LogPath, "replica.log")
		logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_TRUNC, 0666)
		if err == nil {
			logFile.Close()
		}
	}

	raftPeers := make([]raft.Peer, len(config.Peers))
	for i, p := range config.Peers {
		raftPeers[i] = raft.Peer{
			ID:      uint64(i + 1),
			Context: []byte(p),
		}
	}

	n := &node{
		rn:          nil,
		rnLock:      new(sync.Mutex),
		ID:          types.ID(uint64(config.ID)),
		peers:       raftPeers,
		ticker:      time.NewTicker(config.TickTime),
		config:      config,
		kvApp:       config.KVApp,
		state:       newNodeState(),
		storage:     raft.NewMemoryStorage(),
		storageLock: new(sync.Mutex),
		logger:      zap.NewExample(),
		doneChan:    make(chan struct{}),
	}

	transport, err := newNetrixTransport(config.TransportConfig, n)
	if err != nil {
		return nil, err
	}
	for _, peer := range raftPeers {
		if peer.ID != uint64(n.ID) {
			transport.AddPeer(peer.ID, []string{string(peer.Context)})
		}
	}
	n.transport = transport

	if err := n.transport.Start(); err != nil {
		log.Fatalf("failed to start transport: %s", err)
	}
	return n, nil
}

func (n *node) SetRN(rn raft.Node) {
	n.rnLock.Lock()
	defer n.rnLock.Unlock()
	n.rn = rn
}

func (n *node) GetRN() raft.Node {
	n.rnLock.Lock()
	defer n.rnLock.Unlock()
	return n.rn
}

func (n *node) setupRaftLogger() raft.Logger {
	var logger *raft.DefaultLogger
	if _, err := os.Stat(n.config.LogPath); err != nil {
		os.MkdirAll(n.config.LogPath, 0750)
	}
	logFilePath := path.Join(n.config.LogPath, "replica.log")
	var logFile *os.File = nil
	if _, err := os.Stat(logFilePath); err != nil {
		logFile, err = os.Create(logFilePath)
		if err != nil {
			logFile = nil
		}
	} else {
		logFile, err = os.OpenFile(logFilePath, os.O_RDWR|os.O_APPEND, 0666)
		if err != nil {
			logFile = nil
		}
	}

	if logFile != nil {
		logger = &raft.DefaultLogger{
			Logger: log.New(logFile, "raft", log.LstdFlags),
		}
		logger.Info("enabling debug logs")
		logger.EnableDebug()
	} else {
		logger = &raft.DefaultLogger{
			Logger: log.New(os.Stderr, "raft", log.LstdFlags),
		}
	}
	return logger
}

func (n *node) Start() error {
	if n.state.IsRunning() {
		return nil
	}
	n.storage.Reset()
	config := &raft.Config{
		ID:                        uint64(n.ID),
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   n.storage,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
		Logger:                    n.setupRaftLogger(),
	}
	(&raft.DefaultLogger{
		Logger: log.New(os.Stdout, "raft", log.LstdFlags),
	}).Info("starting node")
	n.SetRN(raft.StartNode(config, n.peers))
	n.state.SetRunning(true)
	go n.raftloop()
	return nil
}

func (n *node) Stop() error {
	if !n.state.IsRunning() {
		return nil
	}
	n.doneChan <- struct{}{}
	n.GetRN().Stop()
	n.state.SetRunning(false)
	return nil
}

func (n *node) raftloop() {
	for {
		if !n.state.IsRunning() {
			continue
		}
		select {
		case <-n.ticker.C:
			n.GetRN().Tick()
			nodeState := n.GetRN().Status()
			PublishEventToNetrix("State", map[string]string{
				"state": nodeState.String(),
			})
			n.state.UpdateRaftState(nodeState.RaftState)
			if n.state.UpdateTermState(nodeState.Term) {
				PublishEventToNetrix("TermChange", map[string]string{
					"term": strconv.FormatUint(nodeState.Term, 10),
				})
			}
		case rd := <-n.GetRN().Ready():
			if n.state.UpdateTermState(rd.Term) {
				PublishEventToNetrix("TermChange", map[string]string{
					"term": strconv.FormatUint(rd.Term, 10),
				})
			}
			if !raft.IsEmptySnap(rd.Snapshot) {
				n.storage.ApplySnapshot(rd.Snapshot)
			}
			n.storage.Append(rd.Entries)
			n.sendMessages(rd.Messages)
			n.applyEntries(rd.CommittedEntries)
			n.GetRN().Advance()
		case <-n.doneChan:
			return
		}
	}
}

func (n *node) sendMessages(msgs []raftpb.Message) {
	for i := 0; i < len(msgs); i++ {
		if msgs[i].Type == raftpb.MsgSnap {
			msgs[i].Snapshot.Metadata.ConfState = n.state.ConfState()
		}
	}
	n.transport.Send(msgs)
}

func (n *node) applyEntries(entries []raftpb.Entry) {
	if len(entries) == 0 {
		return
	}
	commitIndex := n.state.CommitIndex()
	if entries[0].Index > commitIndex+1 {
		log.Fatal("committed entry is too big")
		return
	}
	if commitIndex-entries[0].Index+1 < uint64(len(entries)) {
		entries = entries[commitIndex-entries[0].Index+1:]
	}

	for _, entry := range entries {
		PublishEventToNetrix("Commit", map[string]string{
			"replica": n.ID.String(),
			"entry":   string(entry.Data),
			"index":   strconv.Itoa(int(entry.Index)),
		})
		n.state.UpdateCommitIndex(entry.Index)
		switch entry.Type {
		case raftpb.EntryNormal:
			if len(entry.Data) == 0 {
				break
			}
			n.kvApp.Set(string(entry.Data))
		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			cc.Unmarshal(entry.Data)
			n.state.UpdateConfState(*n.GetRN().ApplyConfChange(cc))
			switch cc.Type {
			case raftpb.ConfChangeAddNode:
				if len(cc.Context) > 0 {
					n.transport.AddPeer(cc.NodeID, []string{string(cc.Context)})
				}
			case raftpb.ConfChangeRemoveNode:
				if cc.NodeID == uint64(n.ID) {
					log.Println("I've been removed from the cluster! Shutting down.")
					n.Stop()
					return
				}
				n.transport.RemovePeer(cc.NodeID)
			}
		}
	}
}

func (n *node) Restart() error {
	n.Stop()
	if err := n.ResetStorage(); err != nil {
		return fmt.Errorf("failed to reset storage: %s", err)
	}
	return n.Start()
}

func (n *node) ResetStorage() error {
	if n.wal != nil {
		n.wal.Close()
	}
	n.storage.Reset()
	return os.RemoveAll(n.config.StorageDir)
}

func (n *node) Process(ctx context.Context, m raftpb.Message) error {
	if !n.state.IsRunning() {
		return nil
	}
	rn := n.GetRN()

	return rn.Step(ctx, m)
}

func (n *node) Propose(data []byte) error {
	return n.GetRN().Propose(context.TODO(), data)
}

func (n *node) ProposeConfChange(cc raftpb.ConfChange) {
	n.GetRN().ProposeConfChange(context.TODO(), cc)
}

func (n *node) Ready() bool {
	return n.GetRN() != nil
}
