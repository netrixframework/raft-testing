package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"sync"
)

type kvApp struct {
	kv   map[string]string
	lock *sync.Mutex
}

type kv struct {
	Key string
	Val string
}

func newKVApp() *kvApp {
	return &kvApp{
		kv:   make(map[string]string),
		lock: new(sync.Mutex),
	}
}

func (a *kvApp) Get(key string) (string, bool) {
	a.lock.Lock()
	defer a.lock.Unlock()
	v, ok := a.kv[key]
	return v, ok
}

func (a *kvApp) Set(data string) bool {
	var keyValue kv
	if err := gob.NewDecoder(bytes.NewBufferString(data)).Decode(&keyValue); err != nil {
		return false
	}
	a.lock.Lock()
	a.kv[keyValue.Key] = keyValue.Val
	a.lock.Unlock()
	return true
}

func (a *kvApp) Snapshot() ([]byte, error) {
	a.lock.Lock()
	defer a.lock.Unlock()
	return json.Marshal(a.kv)
}

func (a *kvApp) Restore(data []byte) error {
	var kv map[string]string
	if err := json.Unmarshal(data, &kv); err != nil {
		return err
	}
	a.lock.Lock()
	a.kv = kv
	a.lock.Unlock()
	return nil
}
