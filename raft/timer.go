package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	netrixclient "github.com/netrixframework/go-clientlibrary"
	raft "github.com/netrixframework/raft-testing/raft/protocol"
	raftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

var (
	globalRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type Timer interface {
	Track(m raftpb.Message)
	UpdateState(raft.Status)
	Start()
	Stop()
	Poll() chan bool
	Reset()
}

type NormalTimer struct {
	state           raft.StateType
	heardFromLeader bool
	lock            *sync.Mutex
	timeout         time.Duration
	outChan         chan bool
	doneChan        chan bool
	resetChan       chan bool
}

var _ Timer = &NormalTimer{}

func NewNormalTimer(duration time.Duration) *NormalTimer {
	return &NormalTimer{
		state:           raft.StateFollower,
		heardFromLeader: false,
		lock:            new(sync.Mutex),
		timeout:         duration,
		outChan:         make(chan bool, 1),
		doneChan:        make(chan bool, 1),
		resetChan:       make(chan bool, 1),
	}
}

func (t *NormalTimer) Track(m raftpb.Message) {
	t.lock.Lock()
	state := t.state
	t.lock.Unlock()
	if state == raft.StateLeader {
		return
	}
	switch m.Type {
	case raftpb.MsgApp:
		fallthrough
	case raftpb.MsgHeartbeat:
		fallthrough
	case raftpb.MsgSnap:
		t.setHeard()
	}
}

func (t *NormalTimer) setHeard() {
	t.lock.Lock()
	t.heardFromLeader = true
	t.lock.Unlock()
}

func (t *NormalTimer) UpdateState(s raft.Status) {
	t.lock.Lock()
	t.state = s.RaftState
	t.lock.Unlock()
}

func (t *NormalTimer) Start() {
	go t.loop()
}

func (t *NormalTimer) Stop() {
	close(t.doneChan)
}

func (t *NormalTimer) loop() {
	for {
		randomTimeout := t.timeout + time.Duration(
			globalRand.Int63n(t.timeout.Milliseconds())*int64(time.Millisecond),
		)
		select {
		case <-time.After(randomTimeout):
			t.lock.Lock()
			heardFromLeader := t.heardFromLeader
			t.lock.Unlock()
			if !heardFromLeader {
				t.fire()
			}
			t.lock.Lock()
			t.heardFromLeader = false
			t.lock.Unlock()
		case <-t.doneChan:
			return
		case <-t.resetChan:
		}
	}
}

func (t *NormalTimer) fire() {
	t.lock.Lock()
	isLeader := t.state == raft.StateLeader
	t.lock.Unlock()
	if isLeader {
		return
	}
	t.outChan <- true
}

func (t *NormalTimer) Poll() chan bool {
	return t.outChan
}

func (t *NormalTimer) Reset() {
	for {
		remaining := len(t.outChan)
		if remaining == 0 {
			break
		}
		<-t.outChan
	}
	t.resetChan <- true
}

type timeoutInfo struct {
	Term uint64
	Ctr  int
	D    time.Duration
}

var _ netrixclient.TimeoutInfo = &timeoutInfo{}

func (t *timeoutInfo) Key() string {
	return fmt.Sprintf("t_%d_%d", t.Term, t.Ctr)
}

func (t *timeoutInfo) Duration() time.Duration {
	return t.D
}

type NetrixTimer struct {
	client          *netrixclient.ReplicaClient
	netrixChan      chan netrixclient.TimeoutInfo
	outChan         chan bool
	resetChan       chan bool
	doneChan        chan bool
	curTerm         uint64
	heardFromLeader bool
	state           raft.StateType
	timeout         time.Duration
	ctr             int
	lock            *sync.Mutex
}

var _ Timer = &NetrixTimer{}

func NewNetrixTimer(timeout time.Duration) (*NetrixTimer, error) {
	client, err := netrixclient.GetClient()
	if err != nil {
		return nil, err
	}
	return &NetrixTimer{
		client:          client,
		netrixChan:      client.TimeoutChan(),
		outChan:         make(chan bool, 1),
		resetChan:       make(chan bool, 1),
		doneChan:        make(chan bool, 1),
		curTerm:         0,
		heardFromLeader: false,
		state:           raft.StateFollower,
		timeout:         timeout,
		ctr:             0,
		lock:            new(sync.Mutex),
	}, nil
}

func (t *NetrixTimer) Poll() chan bool {
	return t.outChan
}

func (t *NetrixTimer) Track(m raftpb.Message) {
	t.lock.Lock()
	state := t.state
	t.lock.Unlock()
	if state == raft.StateLeader {
		return
	}
	switch m.Type {
	case raftpb.MsgApp:
		fallthrough
	case raftpb.MsgHeartbeat:
		fallthrough
	case raftpb.MsgSnap:
		t.setHeard()
	}
}

func (t *NetrixTimer) setHeard() {
	t.lock.Lock()
	t.heardFromLeader = true
	t.lock.Unlock()
}

func (t *NetrixTimer) UpdateState(s raft.Status) {
	t.lock.Lock()
	t.state = s.RaftState
	if s.Term > t.curTerm {
		t.curTerm = s.Term
		t.ctr = 0
	}
	t.lock.Unlock()
}

func (t *NetrixTimer) Start() {
	go t.loop()
}

func (t *NetrixTimer) loop() {
	for {
		randomTimeout := t.timeout + time.Duration(
			globalRand.Int63n(t.timeout.Milliseconds())*int64(time.Millisecond),
		)
		t.lock.Lock()
		term := t.curTerm
		ctr := t.ctr
		t.ctr = t.ctr + 1
		t.lock.Unlock()
		t.client.StartTimer(&timeoutInfo{
			Term: term,
			Ctr:  ctr,
			D:    randomTimeout,
		})
		select {
		case timeout := <-t.netrixChan:
			t.lock.Lock()
			heardFromLeader := t.heardFromLeader
			t.lock.Unlock()
			if !heardFromLeader {
				t.fire(timeout.(*timeoutInfo))
			}
			t.lock.Lock()
			t.heardFromLeader = false
			t.lock.Unlock()
		case <-t.doneChan:
			return
		case <-t.resetChan:
		}
	}
}

func (t *NetrixTimer) fire(timeout *timeoutInfo) {
	t.lock.Lock()
	curTerm := t.curTerm
	state := t.state
	t.lock.Unlock()
	if state == raft.StateLeader || timeout.Term < curTerm {
		return
	}
	t.outChan <- true
}

func (t *NetrixTimer) Stop() {
	close(t.doneChan)
}

func (t *NetrixTimer) Reset() {
	for {
		remaining := len(t.outChan)
		if remaining == 0 {
			break
		}
		<-t.outChan
	}
	t.resetChan <- true
}
