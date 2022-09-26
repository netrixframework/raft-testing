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
	rn     raft.Node
	rnLock *sync.Mutex
	ID     types.ID
	peers  []raft.Peer
	config *nodeConfig
	ticker *time.Ticker

	// timer       Timer
	// timerChan   chan bool
	storage     *raft.MemoryStorage
	storageLock *sync.Mutex
	snapshotter *snap.Snapshotter
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
	// log.Printf(
	// 	"starting node with %d peers: %s",
	// 	len(config.Peers),
	// 	strings.Join(config.Peers, ","),
	// )
	// Truncating the log file before every new node creation
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
	// timer, err := NewNetrixTimer(10 * config.TickTime)
	// if err != nil {
	// 	return nil, err
	// }
	// n.timer = timer
	// n.timerChan = timer.Poll()

	if err := n.transport.Start(); err != nil {
		log.Fatalf("failed to start transport: %s", err)
	}

	// timer.Start()

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
	snapPath := path.Join(n.config.StorageDir, "snapshots")
	if _, err := os.Stat(n.config.StorageDir); err != nil {
		if err := os.Mkdir(n.config.StorageDir, 0750); err != nil {
			log.Fatalf("failed to create storage dir: %s", err)
		}
		if err := os.Mkdir(snapPath, 0750); err != nil {
			log.Fatalf("failed to create snapshot dir: %s", err)
		}
	}

	if _, err := os.Stat(snapPath); err != nil {
		if err := os.Mkdir(snapPath, 0750); err != nil {
			log.Fatalf("failed to create snapshot dir: %s", err)
		}
	}
	n.snapshotter = snap.New(n.logger, snapPath)
}

func (n *node) setupWAL() bool {
	walPath := path.Join(n.config.StorageDir, "wal")
	var snapshot *raftpb.Snapshot = nil
	if _, err := os.Stat(n.config.StorageDir); err != nil {
		if err := os.Mkdir(n.config.StorageDir, 0750); err != nil {
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
	} else {
		if err := os.Mkdir(walPath, 0750); err != nil {
			log.Fatalf("failed to create wal dir: %s", err)
		}
		w, err := wal.Create(n.logger, walPath, nil)
		if err != nil {
			log.Fatalf("failed to create wal: %s", err)
		}
		w.Close()
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
	_, hs, entries, err := n.wal.ReadAll()
	if err != nil {
		log.Fatalf("failed to read from wal: %s", err)
	}
	if snapshot != nil {
		n.restoreSnapshot()
	}
	n.storage.SetHardState(hs)
	n.storage.Append(entries)
	return snapshot != nil
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
	// Figure out to start or restart node
	// n.setupSnapshotter()
	// restart := n.setupWAL()
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

	// if restart {
	// 	n.SetRN(raft.RestartNode(config))
	// } else {
	(&raft.DefaultLogger{
		Logger: log.New(os.Stdout, "raft", log.LstdFlags),
	}).Info("starting node")
	n.SetRN(raft.StartNode(config, n.peers))
	// }
	// Start node
	n.state.SetRunning(true)
	// n.timer.Reset()
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
			// n.timer.UpdateState(nodeState)
			newState := n.state.UpdateRaftState(nodeState.RaftState)
			if newState {
				PublishEventToNetrix("StateChange", map[string]string{
					"new_state": nodeState.RaftState.String(),
					"term":      strconv.FormatUint(nodeState.Term, 10),
				})
				// if nodeState.RaftState == raft.StateLeader {
				// 	var buf bytes.Buffer
				// 	if err := gob.NewEncoder(&buf).Encode(kv{"test", "test"}); err == nil {
				// 		n.Propose(buf.Bytes())
				// 	}
				// }
			}
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
			// n.wal.Save(rd.HardState, rd.Entries)
			if !raft.IsEmptySnap(rd.Snapshot) {
				n.storage.ApplySnapshot(rd.Snapshot)
				// n.saveSnapshot(rd.Snapshot)
				// n.restoreSnapshot()
			}
			n.storage.Append(rd.Entries)
			n.sendMessages(rd.Messages)
			n.applyEntries(rd.CommittedEntries)
			// n.takeSnapshot()
			n.GetRN().Advance()
		// case <-n.timerChan:
		// 	n.GetRN().Campaign(context.TODO())
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
	// n.timer.Track(m)
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
