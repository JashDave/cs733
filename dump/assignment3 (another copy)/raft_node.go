package raftnode

import (
	//	"fmt"
	"math"
	"time"
	//	"errors"
	"encoding/json"
	raftsm "github.com/JashDave/cs733/assignment3/assignment2"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/log"
	"io/ioutil"
	"strconv"
)

type Config struct {
	Cluster          []NetConfig
	Id               uint64
	LogDir           string
	ElectionTimeout  uint64
	HeartbeatTimeout uint64
}

type NetConfig struct {
	Id   uint64
	Host string
	Port uint16
}

type TermVotedFor struct {
	CurrentTerm uint64
	VotedFor    uint64
}

type CommitInfo struct {
	Data  []byte
	Index uint64
	Err   error
}

type RaftNode struct {
	id          uint64
	majority    uint64
	currentTerm uint64
	votedFor    uint64
	commitIndex uint64
	leaderId    uint64

	rnlog      *log.Log
	server     cluster.Server
	commitChan chan CommitInfo

	sm           *raftsm.StateMachine
	smActionChan chan raftsm.Action
	smEventChan  chan raftsm.Event
}

func GetServer(id int, Cluster []NetConfig) (cluster.Server, error) {
	peers := make([]cluster.PeerConfig, len(Cluster))
	for i, e := range Cluster {
		peers[i] = cluster.PeerConfig{Id: int(e.Id), Address: e.Host + ":" + strconv.Itoa(int(e.Port))}

	}
	cnf := cluster.Config{Peers: peers}
	return cluster.New(id, cnf)
}

func CreateRaftNode(conf Config) *RaftNode {
	rn := new(RaftNode)

	rn.leaderId = 0    //? 0 is invalid
	rn.commitIndex = 0 //?verify
	rnlog, err := log.Open(conf.LogDir)
	if err != nil {
		return nil
	}
	rn.rnlog = rnlog

	rn.server, err = GetServer(int(conf.Id), conf.Cluster)
	if err != nil {
		return nil
	}

	//--Init State Machine ---
	peerIds := make([]uint64, len(conf.Cluster))
	for i, e := range conf.Cluster {
		if e.Id != conf.Id {
			peerIds[i] = e.Id
		}
	}
	rn.majority = uint64(math.Ceil(float64(len(conf.Cluster)) / 2.0)) //? Assumes len is always odd
	filename := "TermVotedFor"                                        //? change required
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil //error
	}
	var tvf TermVotedFor
	err = json.Unmarshal(data, tvf)
	if err != nil {
		return nil //error
	}
	rn.currentTerm = tvf.CurrentTerm
	rn.votedFor = tvf.VotedFor
	//le := make([]raftsm.LogEntry{},1) //? initilize from Log
	rn.sm = raftsm.InitStateMachine(conf.Id, peerIds, rn.majority, time.Duration(conf.ElectionTimeout)*time.Millisecond, time.Duration(conf.HeartbeatTimeout)*time.Millisecond, rn.currentTerm, rn.votedFor /*//?*/, []raftsm.LogEntry{raftsm.LogEntry{0, nil, false}})

	rn.smActionChan = *rn.sm.GetActionChannel()
	rn.smEventChan = *rn.sm.GetEventChannel()
	rn.commitChan = make(chan CommitInfo, 100)
	return rn
}

func (rn *RaftNode) Append(data []byte) {
	rn.smEventChan <- raftsm.CreateEvent("Append", "data", data)
}

func (rn *RaftNode) GetCommitChannel() *chan CommitInfo {
	return &rn.commitChan
}

func (rn *RaftNode) CommittedIndex() uint64 {
	return rn.commitIndex
}

func (rn *RaftNode) Get(index uint64) (error, []byte) {
	data, err := rn.rnlog.Get(int64(index))
	if err != nil {
		return err, nil
	}
	var le raftsm.LogEntry
	err = json.Unmarshal(data, le)
	if err != nil {
		return err, nil
	}
	return nil, le.Data
}

func (rn *RaftNode) Id() uint64 {
	return rn.id
}

func (rn *RaftNode) LeaderId() uint64 {
	return rn.leaderId
}

func (rn *RaftNode) Shutdown() {
	rn.rnlog.Close()
	rn.sm.Stop()
	//Stop the machine
}
