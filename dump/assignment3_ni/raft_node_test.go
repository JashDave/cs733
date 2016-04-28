package raftnode

import (
	"fmt"
	"github.com/cs733-iitb/cluster/mock"
	"os"
	"strconv"
	"testing"
	"time"
)

const DIRNAME string = "TestLog"

var ids []int = []int{100, 200, 300, 400, 500}
var nc []NetConfig = []NetConfig{{uint64(ids[0]), "localhost", uint16(5001)},
	{uint64(ids[1]), "localhost", uint16(5002)},
	{uint64(ids[2]), "localhost", uint16(5003)},
	{uint64(ids[3]), "localhost", uint16(5004)},
	{uint64(ids[4]), "localhost", uint16(5005)}}

const EleTimeout uint64 = 1000
const HBTimeout uint64 = 100


func ImportHolder(){
	fmt.Println("")
}

func GetNode(idx int,t *testing.T) *RaftNode {
	conf := Config{nc, uint64(ids[idx]), DIRNAME + strconv.Itoa(int(ids[idx])), EleTimeout, HBTimeout}
	rn,err := CreateRaftNode(conf)
	if err!= nil {
		t.Fatal("Error creating node.", err)
	}
	return rn
}

func MakeNodes(t *testing.T) ([]*RaftNode, *mock.MockCluster) {
	cl, _ := mock.NewCluster(nil)
	rafts := make([]*RaftNode, len(nc))
	for i := 0; i < len(nc); i++ {
		rafts[i] = GetNode(i,t)
		rafts[i].server.Close() //Override with mock cluster
		tmp, err := cl.AddServer(int(rafts[i].id))
		if err != nil {
			t.Fatal("Error creating nodes.", err)
		}
		rafts[i].server = tmp
	}
	return rafts, cl
}

func Start(rafts []*RaftNode) {
	for i := 0; i < len(rafts); i++ {
		rafts[i].Start()
	}
}

func Stop(rafts []*RaftNode) {
	for i := 0; i < len(rafts); i++ {
		rafts[i].Shutdown()
	}
}

func ClearAll() {
	for i := 0; i < len(nc); i++ {
		os.RemoveAll(DIRNAME + strconv.Itoa(int(ids[i])))
	}
}

func GetMajorityLeader(rafts []*RaftNode) *RaftNode {
	majority := int(len(rafts)/2) + 1
	ldr_count := make([]int, len(rafts))
	for _, e := range rafts {
		ldr := e.LeaderId()
		if ldr != 0 {
			ldr_count[int(ldr/100)-1]++
			if ldr_count[int(ldr/100)-1] >= majority {
				return rafts[int(ldr/100)-1]
			}
		}
	}
	return nil
}

func WaitToGetLeader(rafts []*RaftNode, times int) *RaftNode {
	ldr := GetMajorityLeader(rafts)
	for i := 0; i < times && ldr == nil; i++ {
		time.Sleep(time.Duration(EleTimeout) * time.Millisecond)
		ldr = GetMajorityLeader(rafts)
	}
	return ldr
}
/*
func TestInit(t *testing.T) {
	ClearAll()
	rn := GetNode(0,t)
	if rn == nil {
		t.Error("Error creating Raft Node")
	}
	rn.Shutdown()
}


func TestCommit(t *testing.T) {
	rn, _ := MakeNodes(t)
	Start(rn)
	defer Stop(rn)
	//Get Leader
	ldr := WaitToGetLeader(rn, 10)

	if ldr == nil {
		t.Fatal("Error getting leader")
	}
	testdata := "1234567890"
	ldr.Append([]byte(testdata))

	time.Sleep(3 * time.Second)
	for _, node := range rn {
		select {
		case ci := <- *node.GetCommitChannel():
			if ci.Err != nil {
				t.Fatal(ci.Err)
			}
			if string(ci.Data) != testdata {
				t.Fatal("Got different data")
			}
			//Check Log also
			commitIndex := node.CommittedIndex() 
			err,data := node.Get(commitIndex)
			if err!=nil {
				t.Fatal("Not commited in log",err)
			}
			if string(data) != testdata {
				t.Fatal("Got different data from log")
			}
		default:
			t.Fatal("Expected message on all nodes")
		}
	}
}

//If only majority of nodes are in partition then the partition must get its leader
func TestMajorityPartition(t *testing.T) {
	rn, ci := MakeNodes(t)
	majority := int(len(rn)/2) + 1
	ci.Partition(ids[:majority],ids[majority:])
	Start(rn)
	defer Stop(rn)
	//Get Leader
	ldr := WaitToGetLeader(rn, 10)

	if ldr == nil {
		t.Fatal("Error getting leader in partition")
	}
	testdata := "!@#$%^&*()"
	ldr.Append([]byte(testdata))

	//Commit for all nodes in majority partition
	time.Sleep(3 * time.Second)
	for _, node := range rn[:majority] {
		select {
		case ci := <- *node.GetCommitChannel():
			if ci.Err != nil {
				t.Fatal(ci.Err)
			}
			if string(ci.Data) != testdata {
				t.Fatal("Got different data")
			}
		default:
			t.Fatal("Expected message on all nodes")
		}
	}

	//No Commit for any nodes in minority partition
	for _, node := range rn[majority:] {
		select {
		case ci := <- *node.GetCommitChannel():
			if ci.Err == nil {
				t.Fatal("Bad Commit")
			}
		default:
		}
	}
}


//No leader must be there if all partitions are smaller than majority
func TestMinorityPartition(t *testing.T) {
	rn, ci := MakeNodes(t)
	majority := int(len(rn)/2) + 1
	ci.Partition(ids[:majority-1],ids[majority-1:2*(majority-1)],ids[2*(majority-1):])
	Start(rn)
	defer Stop(rn)
	//Get Leader
	ldr := WaitToGetLeader(rn, 10)

	if ldr != nil {
		t.Fatal("Error leader in minor partition")
	}
}

//New leader must come up if old leader stops or gets partitioned
func TestLeaderGetsPartitioned(t *testing.T) {
	rn, cl := MakeNodes(t)
	Start(rn)
	defer Stop(rn)
	//Get Leader
	ldr := WaitToGetLeader(rn, 10)
	if ldr == nil {
		t.Fatal("Error getting leader")
	}
	ldridx :=ldr.Id()
	part := make([]int,len(rn)-1)
	i :=0
	for _,e := range rn {
		if ldridx != e.Id() {
			part[i] = int(e.Id())
			i++
		}
	}
	//Partition leader from others
	cl.Partition(part,[]int{int(ldridx)})
	//wait for others to timeout
	time.Sleep(time.Duration(EleTimeout)*2 * time.Millisecond)
	//Get new leader
	ldr = WaitToGetLeader(rn, 10)
	if ldr == nil {
		t.Fatal("Error new getting leader in partition")
	}
	if ldr.Id() == ldridx {
		t.Fatal("Bad(same) leader")
	}
}

//Leader must get elected in healed partition
func TestPartitionGetsHealed(t *testing.T) {
	rn, ci := MakeNodes(t)
	majority := int(len(rn)/2) + 1
	ci.Partition(ids[:majority-1],ids[majority-1:2*(majority-1)],ids[2*(majority-1):])
	Start(rn)
	defer Stop(rn)
	//Get Leader
	ldr := WaitToGetLeader(rn, 10)

	if ldr != nil {
		t.Fatal("Error leader in minor partition")
	}

	ci.Heal()
	//Get Leader
	ldr = WaitToGetLeader(rn, 10)
	if ldr == nil {
		t.Fatal("No leader in healed partition")
	}
}



*/

func TestMultipleAppend(t *testing.T) {
	ClearAll()
	rn, _ := MakeNodes(t)
	Start(rn)
	defer Stop(rn)
	//Get Leader
	ldr := WaitToGetLeader(rn, 10)

	if ldr == nil {
		t.Fatal("Error getting leader")
	}

	times := 10

	for i:=0;i<times;i++ {
		ldr.Append([]byte(strconv.Itoa(i)))
	}

	for i:=0; i<2*times &&  int(ldr.CommittedIndex()) < (times-1); i++ {
		fmt.Println("i:",i,"CmtIdx:",ldr.CommittedIndex(),"LastLogIdx",ldr.rnlog.GetLastIndex())
		select {
		case ci := <- *ldr.GetCommitChannel():
			fmt.Println("CI",ci)
			if ci.Err != nil {
				t.Fatal(ci.Err)
			}
		default:
		}
		time.Sleep(time.Millisecond * 1000)
	}

	for _,ldr = range rn {
	fmt.Println("NODE ID :",ldr.Id(),"LastLogIndex",ldr.rnlog.GetLastIndex())
		for i:=uint64(0);i<=ldr.CommittedIndex() ;i++ {
			err,data := ldr.Get(i)
			fmt.Println("i:",i,"Data:",string(data))
			if err!=nil {
				t.Fatal("Not commited in log",err)
			}
		}
	}
}

