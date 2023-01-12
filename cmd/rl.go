package cmd

import (
	"bufio"
	"encoding/json"
	"hash"
	"hash/crc32"
	"os"
	"sync"

	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/rl"
	"github.com/netrixframework/netrix/types"
)

type raftInterpreter struct {
	states       map[string]string
	uniqueStates map[string]string
	recordPath   string
	lock         *sync.Mutex
	hash         hash.Hash
}

func newRaftInterpreter(filePath string) *raftInterpreter {
	return &raftInterpreter{
		states:       make(map[string]string),
		uniqueStates: make(map[string]string),
		recordPath:   filePath,
		lock:         new(sync.Mutex),
		hash:         crc32.New(crc32.MakeTable(crc32.IEEE)),
	}
}

var _ rl.Interpreter = &raftInterpreter{}

func (r *raftInterpreter) CurState() rl.State {
	r.lock.Lock()
	defer r.lock.Unlock()

	return &raftState{
		states: r.states,
		hash:   r.hash,
	}
}

func (r *raftInterpreter) Update(e *types.Event, ctx *strategies.Context) {
	if !e.IsGeneric() {
		return
	}
	eType := e.Type.(*types.GenericEventType)
	if eType.T != "State" {
		return
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	r.states[string(e.Replica)] = eType.Params["state"]

	s := &raftState{states: r.states, hash: r.hash}
	stateKey := s.Hash()
	if _, ok := r.uniqueStates[stateKey]; !ok {
		r.uniqueStates[stateKey] = s.String()
	}
}

func (r *raftInterpreter) Reset() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.states = make(map[string]string)
}

func (r *raftInterpreter) CoveredStates() int {
	r.lock.Lock()
	defer r.lock.Unlock()
	return len(r.uniqueStates)
}

func (r *raftInterpreter) RecordCoverage() error {
	file, err := os.Create(r.recordPath)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(file)

	r.lock.Lock()
	for _, k := range r.uniqueStates {
		writer.WriteString(k + "\n")
	}
	r.lock.Unlock()
	writer.Flush()
	file.Close()
	return nil
}

type raftState struct {
	states map[string]string
	hash   hash.Hash
}

func (s *raftState) Hash() string {
	b, err := json.Marshal(s.states)
	if err != nil {
		return ""
	}
	return string(s.hash.Sum(b))
}

func (s *raftState) String() string {
	b, err := json.Marshal(s.states)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *raftState) Actions() []*strategies.Action {
	return []*strategies.Action{}
}
