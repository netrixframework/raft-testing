package main

import (
	"sync"

	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type nodeState struct {
	snapshotIndex uint64
	commitIndex   uint64
	confState     raftpb.ConfState
	term          uint64
	lock          *sync.Mutex

	raftState raft.StateType
	running   bool
	runLock   *sync.Mutex
}

func newNodeState() *nodeState {
	return &nodeState{
		snapshotIndex: 0,
		commitIndex:   0,
		confState:     raftpb.ConfState{},
		term:          0,
		raftState:     raft.StateFollower,
		running:       false,
		runLock:       new(sync.Mutex),
		lock:          new(sync.Mutex),
	}
}

func (s *nodeState) UpdateSnapshotIndex(index uint64) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	new := s.snapshotIndex != index
	s.snapshotIndex = index
	return new
}

func (s *nodeState) UpdateCommitIndex(index uint64) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	new := s.commitIndex != index
	s.commitIndex = index
	return new
}

func (s *nodeState) UpdateConfState(confState raftpb.ConfState) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.confState = confState
}

func (s *nodeState) UpdateRaftState(t raft.StateType) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	new := s.raftState != t
	s.raftState = t
	return new
}

func (s *nodeState) UpdateTermState(term uint64) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	new := s.term != term
	s.term = term
	return new
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

func (s *nodeState) Term() uint64 {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.term
}

func (s *nodeState) RaftState() raft.StateType {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.raftState
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
