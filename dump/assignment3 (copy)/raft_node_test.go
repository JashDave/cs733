package raftnode

import (
	"testing"
	"fmt"
	"time"
	"strconv"
)

func GetNode(id uint64) (*RaftNode) {
	nc := []NetConfig{{uint64(100), "localhost", uint16(5001)}, {uint64(200), "localhost", uint16(5002)}, {uint64(300), "localhost", uint16(5003)}, {uint64(400), "localhost", uint16(5004)}, {uint64(500), "localhost", uint16(5005)}}
	conf := Config{nc, id, "log"+strconv.Itoa(int(id)), uint64(5000), uint64(1000)}
	rn := CreateRaftNode(conf)
	return rn
}


func TestInit(t *testing.T) {
	rn := GetNode(100)
	if rn == nil {
		t.Error("Error creating Raft Node")
	}
	rn.Shutdown()
}



func TestAll(t *testing.T) {
	rn := make([]*RaftNode,5)
	for i:=0;i<5;i++ {
		rn[i] = GetNode(uint64(100*(i+1)))
		if rn[i] == nil {
			t.Error("Error creating Raft Node",i*100)
		} else {
			rn[i].Start()
		}
	}

	//Get Leader
	L:= uint64(0)	
for (L==0) {
	time.Sleep(5*time.Second)
L = rn[0].LeaderId()
}

	time.Sleep(2*time.Second)
	
	for i:=0;i<100;i++ {
fmt.Println("I:",i)
		//time.Sleep(1000000*time.Microsecond)
		rn[int(L/100)-1].Append([]byte("12345"))
	}


	time.Sleep(100*time.Second)
}

