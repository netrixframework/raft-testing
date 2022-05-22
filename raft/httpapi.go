package main

import (
	"bytes"
	"encoding/gob"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"

	"go.etcd.io/etcd/raft/v3/raftpb"
)

type httpKVAPI struct {
	store *kvApp
	node  *node

	proposeC    chan []byte
	confChangeC chan raftpb.ConfChange

	doneChan chan struct{}
	doneOnce *sync.Once
}

func newHTTPKVAPI(store *kvApp, node *node) *httpKVAPI {
	return &httpKVAPI{
		store:       store,
		node:        node,
		proposeC:    make(chan []byte, 10),
		confChangeC: make(chan raftpb.ConfChange, 10),
		doneChan:    make(chan struct{}),
		doneOnce:    new(sync.Once),
	}
}

func (a *httpKVAPI) Start() {
	go a.poll()
}

func (a *httpKVAPI) Stop() {
	a.doneOnce.Do(func() {
		close(a.doneChan)
	})
}

func (a *httpKVAPI) poll() {
	for {
		select {
		case prop := <-a.proposeC:
			a.node.Propose(prop)
		case confChange := <-a.confChangeC:
			a.node.ProposeConfChange(confChange)
		case <-a.doneChan:
			return
		}
	}
}

func (h *httpKVAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.RequestURI
	defer r.Body.Close()
	switch r.Method {
	case http.MethodPut:
		v, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Failed to read on PUT (%v)\n", err)
			http.Error(w, "Failed on PUT", http.StatusBadRequest)
			return
		}

		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(kv{key, string(v)}); err == nil {
			h.proposeC <- buf.Bytes()
		}

		w.WriteHeader(http.StatusNoContent)
	case http.MethodGet:
		if v, ok := h.store.Get(key); ok {
			w.Write([]byte(v))
		} else {
			http.Error(w, "Failed to GET", http.StatusNotFound)
		}
	case http.MethodPost:
		url, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Failed to read on POST (%v)\n", err)
			http.Error(w, "Failed on POST", http.StatusBadRequest)
			return
		}

		nodeId, err := strconv.ParseUint(key[1:], 0, 64)
		if err != nil {
			log.Printf("Failed to convert ID for conf change (%v)\n", err)
			http.Error(w, "Failed on POST", http.StatusBadRequest)
			return
		}

		h.confChangeC <- raftpb.ConfChange{
			Type:    raftpb.ConfChangeAddNode,
			NodeID:  nodeId,
			Context: url,
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		nodeId, err := strconv.ParseUint(key[1:], 0, 64)
		if err != nil {
			log.Printf("Failed to convert ID for conf change (%v)\n", err)
			http.Error(w, "Failed on DELETE", http.StatusBadRequest)
			return
		}

		h.confChangeC <- raftpb.ConfChange{
			Type:   raftpb.ConfChangeRemoveNode,
			NodeID: nodeId,
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", http.MethodPut)
		w.Header().Add("Allow", http.MethodGet)
		w.Header().Add("Allow", http.MethodPost)
		w.Header().Add("Allow", http.MethodDelete)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
