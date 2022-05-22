package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	netrixclient "github.com/netrixframework/go-clientlibrary"
	ntypes "github.com/netrixframework/netrix/types"
	etypes "go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
)

type netrixTransport struct {
	ID           ntypes.ReplicaID
	netrixClient *netrixclient.ReplicaClient
	node         *node
	msgChan      chan raftpb.Message
	doneCh       chan struct{}
	doneOnce     *sync.Once
}

func newNetrixTransport(config *netrixclient.Config, node *node) (*netrixTransport, error) {
	if err := netrixclient.Init(
		config,
		node,
		nil,
	); err != nil {
		return nil, err
	}
	client, _ := netrixclient.GetClient()
	return &netrixTransport{
		ID:           config.ReplicaID,
		netrixClient: client,
		node:         node,
		msgChan:      make(chan raftpb.Message, 30),
		doneCh:       make(chan struct{}),
		doneOnce:     new(sync.Once),
	}, nil
}

func (t *netrixTransport) Start() error {
	if err := t.netrixClient.Start(); err != nil {
		return fmt.Errorf("failed to start transport: %s", err)
	}
	t.netrixClient.Ready()
	go t.poll()
	return nil
}

func (t *netrixTransport) poll() {
	for {
		select {
		case <-t.doneCh:
			return
		case msg := <-t.msgChan:
			go t.sendMessage(msg)
		default:
			m, ok := t.netrixClient.ReceiveMessage()
			if ok {
				go t.process(m)
			}
		}
	}
}

func (t *netrixTransport) process(m *ntypes.Message) {
	var msg raftpb.Message
	if err := msg.Unmarshal(m.Data); err == nil {
		ctx := context.Background()
		t.node.Process(ctx, msg)
	}
}

func (t *netrixTransport) Handler() http.Handler {
	mux := http.NewServeMux()
	return mux
}

func (t *netrixTransport) Send(msgs []raftpb.Message) {
	for _, msg := range msgs {
		t.msgChan <- msg
	}
}

func (t *netrixTransport) sendMessage(msg raftpb.Message) {
	b, err := msg.Marshal()
	if err != nil {
		return
	}
	t.netrixClient.SendMessage(
		msg.Type.String(),
		ntypes.ReplicaID(strconv.FormatUint(msg.To, 10)),
		b,
		true,
	)
}

func (t *netrixTransport) SendSnapshot(m snap.Message) {

}

func (t *netrixTransport) AddRemote(id etypes.ID, urls []string) {

}

func (t *netrixTransport) AddPeer(id etypes.ID, urls []string) {

}

func (t *netrixTransport) RemovePeer(id etypes.ID) {

}

func (t *netrixTransport) RemoveAllPeers() {

}

func (t *netrixTransport) UpdatePeer(id etypes.ID, urls []string) {

}

func (t *netrixTransport) ActiveSince(id etypes.ID) time.Time {
	return time.Now()
}

func (t *netrixTransport) ActivePeers() int {
	return 0
}

func (t *netrixTransport) Stop() {
	t.netrixClient.Stop()
	t.doneOnce.Do(func() {
		close(t.doneCh)
	})
}

func PublishEventToNetrix(t string, params map[string]string) {
	client, err := netrixclient.GetClient()
	if err != nil {
		return
	}
	client.PublishEventAsync(t, params)
}
