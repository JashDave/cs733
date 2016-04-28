package raftnode

import (
	"testing"
)

func TestInit(t *testing.T) {
	nc := []NetConfig{{uint64(100), "localhost", uint16(5001)}, {uint64(200), "localhost", uint16(5002)}, {uint64(300), "localhost", uint16(5003)}, {uint64(400), "localhost", uint16(5004)}, {uint64(500), "localhost", uint16(5005)}}
	conf := Config{nc, uint64(100), "log100", uint64(500), uint64(250)}
	rn := CreateRaftNode(conf)
	if rn != nil {
		t.Error("Error creating Raft Node")
	}
}
