package util

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"

	"github.com/netrixframework/netrix/log"
	"github.com/netrixframework/netrix/testlib"
	"github.com/netrixframework/netrix/types"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type RaftMsgWrapper struct {
	raftpb.Message
}

var _ types.ParsedMessage = &RaftMsgWrapper{}

func (r *RaftMsgWrapper) Clone() types.ParsedMessage {
	return &RaftMsgWrapper{
		Message: r.Message,
	}
}

type RaftMsgParser struct{}

func (RaftMsgParser) Parse(data []byte) (types.ParsedMessage, error) {
	var msg raftpb.Message
	if err := msg.Unmarshal(data); err != nil {
		return nil, err
	}
	return &RaftMsgWrapper{msg}, nil
}

func SetKeyValue(replica *types.Replica, key, value string) error {
	apiAddrI, ok := replica.Info["http_api_addr"]
	if !ok {
		return fmt.Errorf("no http_api_addr key")
	}
	apiAddr, ok := apiAddrI.(string)
	if !ok {
		return fmt.Errorf("invalid http_api_addr key")
	}

	request, err := http.NewRequest(http.MethodPut, "http://"+apiAddr+"/"+key, bytes.NewBufferString(value))
	if err != nil {
		return fmt.Errorf("failed to create request: %s", err)
	}
	client := &http.Client{}
	_, err = client.Do(request)
	if err != nil {
		return fmt.Errorf("request failed: %s", err)
	}
	return nil
}

func GetMessageFromEvent(e *types.Event, c *testlib.Context) (*RaftMsgWrapper, bool) {
	m, ok := c.GetMessage(e)
	if !ok {
		return nil, false
	}
	raftMessage, ok := m.ParsedMessage.(*RaftMsgWrapper)
	return raftMessage, ok
}

func FReplicas() func(*types.Event, *testlib.Context) (int, bool) {
	return func(_ *types.Event, c *testlib.Context) (int, bool) {
		return int((c.Replicas.Cap() - 1) / 2), true
	}
}

func PickRandomReplica() func(*testlib.Context) error {
	return func(ctx *testlib.Context) error {
		r, ok := ctx.Replicas.GetRandom()
		if !ok {
			return errors.New("no replicas to pick random replica")
		}
		ctx.Logger().With(log.LogParams{
			"random_replica": r.ID,
		}).Info("Picked random replica")
		ctx.Vars.Set("_random_replica", string(r.ID))
		return nil
	}
}

func RandomReplica() func(*types.Event, *testlib.Context) (types.ReplicaID, bool) {
	return func(e *types.Event, ctx *testlib.Context) (types.ReplicaID, bool) {
		rID, ok := ctx.Vars.GetString("_random_replica")
		if !ok {
			r, ok := ctx.Replicas.GetRandom()
			if !ok {
				return "", false
			}
			ctx.Logger().With(log.LogParams{
				"random_replica": r.ID,
			}).Info("Picked random replica")
			ctx.Vars.Set("_random_replica", string(r.ID))
			return r.ID, true
		}
		return types.ReplicaID(rID), true
	}
}
