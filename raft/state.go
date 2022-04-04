package main

import (
	"sync"

	"go.etcd.io/etcd/raft/v3/raftpb"
)

type nodeState struct {
	snapshotIndex uint64
	commitIndex   uint64
	confState     raftpb.ConfState
	lock          *sync.Mutex

	running bool
	runLock *sync.Mutex
}

func newNodeState() *nodeState {
	return &nodeState{
		snapshotIndex: 0,
		commitIndex:   0,
		confState:     raftpb.ConfState{},
		running:       false,
		runLock:       new(sync.Mutex),
		lock:          new(sync.Mutex),
	}
}

func (s *nodeState) UpdateSnapshotIndex(index uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.snapshotIndex = index
}

func (s *nodeState) UpdateCommitIndex(index uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.commitIndex = index
}

func (s *nodeState) UpdateConfState(confState raftpb.ConfState) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.confState = confState
}

func (s *nodeState) SnapshotIndex() uint64 {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.snapshotIndex
}

func (s *nodeState) CommitIndex() uint64 {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.commitIndex
}

func (s *nodeState) ConfState() raftpb.ConfState {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.confState
}

func (s *nodeState) IsRunning() bool {
	s.runLock.Lock()
	defer s.runLock.Unlock()
	return s.running
}

func (s *nodeState) SetRunning(v bool) {
	s.runLock.Lock()
	defer s.runLock.Unlock()
	s.running = v
}
