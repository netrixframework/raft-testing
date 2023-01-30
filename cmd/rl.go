package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"

	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/strategies"
	"github.com/netrixframework/netrix/strategies/rl"
	"github.com/netrixframework/netrix/types"
)

type raftInterpreter struct {
	states       map[string]string
	uniqueStates map[string]string
	recordPath   string
	lock         *sync.Mutex
}

func newRaftInterpreter(filePath string) *raftInterpreter {
	return &raftInterpreter{
		states:       make(map[string]string),
		uniqueStates: make(map[string]string),
		recordPath:   filePath,
		lock:         new(sync.Mutex),
	}
}

var _ rl.Interpreter = &raftInterpreter{}

func (r *raftInterpreter) CurState() rl.State {
	states := make(map[string]string)
	r.lock.Lock()
	for k, v := range r.states {
		states[k] = v
	}
	r.lock.Unlock()

	return &raftState{
		states: states,
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
	ctx.Logger.With(log.LogParams{
		"State":   eType.Params["state"],
		"Replica": e.Replica,
	}).Debug("Updating state")
	r.lock.Lock()
	defer r.lock.Unlock()
	r.states[string(e.Replica)] = eType.Params["state"]

	s := &raftState{states: r.states}
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
}

func (s *raftState) Hash() string {
	b, err := json.Marshal(s.states)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
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
