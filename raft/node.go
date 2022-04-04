package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	netrixclient "github.com/netrixframework/go-clientlibrary"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
)

var (
	snapCount    int = 100
	compactDelay int = 100
)

type node struct {
	rn         raft.Node
	ID         types.ID
	peers      []raft.Peer
	storageDir string
	ticker     *time.Ticker

	storage     *raft.MemoryStorage
	snapshotter *snap.Snapshotter
	wal         *wal.WAL
	state       *nodeState

	kvApp     *kvApp
	transport rafthttp.Transporter

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
}

func newNode(config *nodeConfig) (*node, error) {
	raftPeers := make([]raft.Peer, len(config.Peers))
	for i, p := range config.Peers {
		if i+1 != config.ID {
			raftPeers[types.ID(uint64(i+1))] = raft.Peer{
				ID:      uint64(i + 1),
				Context: []byte(p),
			}
		}
	}

	n := &node{
		rn:         nil,
		ID:         types.ID(uint64(config.ID)),
		peers:      raftPeers,
		ticker:     time.NewTicker(config.TickTime),
		storageDir: config.StorageDir,
		kvApp:      config.KVApp,
		state:      newNodeState(),
		storage:    raft.NewMemoryStorage(),
		logger:     zap.NewExample(),
		doneChan:   make(chan struct{}),
	}

	transport, err := newNetrixTransport(config.TransportConfig, n)
	if err != nil {
		return nil, err
	}
	for _, peer := range raftPeers {
		if peer.ID != uint64(n.ID) {
			transport.AddPeer(types.ID(peer.ID), []string{string(peer.Context)})
		}
	}
	n.transport = transport
	if err := n.transport.Start(); err != nil {
		log.Fatalf("failed to start transport: %s", err)
	}
	return n, nil
}

func (n *node) restoreSnapshot() {
	snapshot, err := n.snapshotter.Load()
	if err != nil && err != snap.ErrNoSnapshot {
		log.Fatalf("failed to load snapshot: %s", err)
	}
	if snapshot != nil {
		n.storage.ApplySnapshot(*snapshot)
		if snapshot.Metadata.Index < n.state.CommitIndex() {
			log.Fatal("snapshot index is lower than applied index")
		}
		if err := n.kvApp.Restore(snapshot.Data); err != nil {
			log.Fatalf("failed to restore snapshot: %s", err)
		}
		n.state.UpdateSnapshotIndex(snapshot.Metadata.Index)
		n.state.UpdateCommitIndex(snapshot.Metadata.Index)
		n.state.UpdateConfState(snapshot.Metadata.ConfState)
	}
}

func (n *node) saveSnapshot(snapshot raftpb.Snapshot) error {
	if raft.IsEmptySnap(snapshot) {
		return nil
	}
	if err := n.snapshotter.SaveSnap(snapshot); err != nil {
		log.Fatalf("failed to save snapshot to snapshotter: %s", err)
	}
	if err := n.wal.SaveSnapshot(walpb.Snapshot{
		Index:     snapshot.Metadata.Index,
		Term:      snapshot.Metadata.Term,
		ConfState: &snapshot.Metadata.ConfState,
	}); err != nil {
		log.Fatalf("failed to save snapshot to wal: %s", err)
	}
	return n.wal.ReleaseLockTo(snapshot.Metadata.Index)
}

func (n *node) setupSnapshotter() {
	snapPath := path.Join(n.storageDir, "snapshots")
	if !fileutil.Exist(n.storageDir) {
		if err := os.Mkdir(n.storageDir, 0750); err != nil {
			log.Fatalf("failed to create storage dir: %s", err)
		}
		if err := os.Mkdir(snapPath, 0750); err != nil {
			log.Fatalf("failed to create snapshot dir: %s", err)
		}
	}

	if !fileutil.Exist(snapPath) {
		if err := os.Mkdir(snapPath, 0750); err != nil {
			log.Fatalf("failed to create snapshot dir: %s", err)
		}
	}
	n.snapshotter = snap.New(n.logger, snapPath)
}

func (n *node) setupWAL() bool {
	walPath := path.Join(n.storageDir, "wal")
	var snapshot *raftpb.Snapshot = nil
	if !fileutil.Exist(n.storageDir) {
		if err := os.Mkdir(n.storageDir, 0750); err != nil {
			log.Fatalf("failed to create storage dir: %s", err)
		}
	}
	if wal.Exist(walPath) {
		walSnaps, err := wal.ValidSnapshotEntries(n.logger, walPath)
		if err != nil {
			log.Fatalf("failed to read wal snapshots: %s", err)
		}
		snapshot, err = n.snapshotter.LoadNewestAvailable(walSnaps)
		if err != nil && err != snap.ErrNoSnapshot {
			log.Fatalf("failed to pick newest snapshot: %s", err)
		}
		walSnap := walpb.Snapshot{}
		if snapshot != nil {
			walSnap.Index = snapshot.Metadata.Index
			walSnap.Term = snapshot.Metadata.Term
		}
		w, err := wal.Open(n.logger, walPath, walSnap)
		if err != nil {
			log.Fatalf("failed to open wal: %s", err)
		}
		n.wal = w
		if snapshot != nil {
			n.restoreSnapshot()
		}
	} else {
		if err := os.Mkdir(walPath, 0750); err != nil {
			log.Fatalf("failed to create wal dir: %s", err)
		}
		w, err := wal.Create(n.logger, walPath, nil)
		if err != nil {
			log.Fatalf("failed to create wal: %s", err)
		}
		n.wal = w
	}
	_, hs, entries, err := n.wal.ReadAll()
	if err != nil {
		log.Fatalf("failed to read from wal: %s", err)
	}
	n.storage.SetHardState(hs)
	n.storage.Append(entries)
	return snapshot != nil
}

func (n *node) Start() error {
	if n.state.IsRunning() {
		return nil
	}
	// Figure out to start or restart node
	n.setupSnapshotter()
	restart := n.setupWAL()
	config := &raft.Config{
		ID:                        uint64(n.ID),
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   n.storage,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
	}

	if restart {
		n.rn = raft.RestartNode(config)
	} else {
		n.rn = raft.StartNode(config, n.peers)
	}
	// Start node
	n.state.SetRunning(true)
	go n.raftloop()
	return nil
}

func (n *node) Stop() error {
	n.doneChan <- struct{}{}
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
			n.rn.Tick()
		case rd := <-n.rn.Ready():
			n.wal.Save(rd.HardState, rd.Entries)
			if !raft.IsEmptySnap(rd.Snapshot) {
				n.saveSnapshot(rd.Snapshot)
				n.restoreSnapshot()
			}
			n.sendMessages(rd.Messages)
			n.applyEntries(rd.CommittedEntries)
			n.takeSnapshot()
			n.rn.Advance()
		case <-n.doneChan:
			return
		}
	}
}

func (n *node) takeSnapshot() {
	if n.state.CommitIndex()-n.state.SnapshotIndex() <= uint64(snapCount) {
		return
	}
	snapData, err := n.kvApp.Snapshot()
	if err != nil {
		log.Fatalf("failed to take snapshot: %s", err)
	}
	confState := n.state.ConfState()
	snapshot, err := n.storage.CreateSnapshot(n.state.CommitIndex(), &confState, snapData)
	if err != nil {
		log.Fatalf("failed to create snapshot: %s", err)
	}
	if err = n.saveSnapshot(snapshot); err != nil {
		log.Fatalf("failed to save snapshot: %s", err)
	}
	compactIndex := 1
	if n.state.CommitIndex() > uint64(compactDelay) {
		compactIndex = int(n.state.CommitIndex()) - compactDelay
	}
	if err = n.storage.Compact(uint64(compactIndex)); err != nil {
		log.Fatalf("failed to compact storage: %s", err)
	}
	n.state.UpdateSnapshotIndex(n.state.CommitIndex())
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

	entries = entries[commitIndex-entries[0].Index+1:]

	for _, entry := range entries {
		switch entry.Type {
		case raftpb.EntryNormal:
			if len(entry.Data) == 0 {
				break
			}
			n.kvApp.Set(string(entry.Data))
		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			cc.Unmarshal(entry.Data)
			n.state.UpdateConfState(*n.rn.ApplyConfChange(cc))
			switch cc.Type {
			case raftpb.ConfChangeAddNode:
				if len(cc.Context) > 0 {
					n.transport.AddPeer(types.ID(cc.NodeID), []string{string(cc.Context)})
				}
			case raftpb.ConfChangeRemoveNode:
				if cc.NodeID == uint64(n.ID) {
					log.Println("I've been removed from the cluster! Shutting down.")
					n.Stop()
					return
				}
				n.transport.RemovePeer(types.ID(cc.NodeID))
			}
		}
	}
}

func (n *node) Restart() error {
	n.Stop()
	if err := n.resetStorage(); err != nil {
		return fmt.Errorf("failed to reset storage: %s", err)
	}
	return n.Start()
}

func (n *node) resetStorage() error {
	n.wal.Close()
	return os.RemoveAll(n.storageDir)
}

func (n *node) Process(ctx context.Context, m raftpb.Message) error {
	if !n.state.IsRunning() {
		return nil
	}
	return n.rn.Step(ctx, m)
}

func (n *node) Propose(data []byte) {
	n.rn.Propose(context.TODO(), data)
}

func (n *node) ProposeConfChange(cc raftpb.ConfChange) {
	n.rn.ProposeConfChange(context.TODO(), cc)
}
