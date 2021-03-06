package raftnode

import (
	"fmt"
	"math"
	"time"
	"errors"
	"encoding/json"
	"encoding/gob"
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
	//leaderId    uint64
	logIndex    uint64
	statefile	string
	timer	*time.Timer

	rnlog      *log.Log
	server     cluster.Server
	commitChan chan CommitInfo

	sm           *raftsm.StateMachine
	smActionChan chan raftsm.Action
	smEventChan  chan raftsm.Event
}



func (rn *RaftNode) processInboxEvent() {
	select {
		case e := <- rn.server.Inbox():
			rn.smEventChan <- e.Msg.(raftsm.Event)
	}
}

func (rn *RaftNode) processInboxEvents() {
	for {
		rn.processInboxEvent()
	}
}


func (rn *RaftNode) processAction() {
	select {
	case e := <-rn.smActionChan:
fmt.Println(rn.id,":",e)
		switch e.Name {
		case "LogStore":
			l := raftsm.LogEntry{e.Data["term"].(uint64),e.Data["data"].([]byte),true}
			idx := int64(e.Data["index"].(uint64))-1
			if(idx<=rn.rnlog.GetLastIndex()) {
				rn.rnlog.TruncateToEnd(idx)
			}
			err := rn.rnlog.Append(l)
			if err!=nil {
				//error
			}
		case "SaveState":
			tvf := TermVotedFor{e.Data["currentTerm"].(uint64),e.Data["votedFor"].(uint64)}
			data, err := json.Marshal(tvf)
			if err != nil {
				//return err//error
			}
			err = ioutil.WriteFile(rn.statefile, data, 0777)
			if err != nil {
fmt.Println("File write errorr",err)
				//return err//error
			}
		case "Alarm":
			rn.alarm(e.Data["t"].(time.Duration))
		case "Send":
			id := int(e.Data["peerId"].(uint64))
			rn.server.Outbox() <- &cluster.Envelope{Pid: id, Msg: e.Data["event"].(raftsm.Event)}
		case "Redirect":
			ci := CommitInfo{nil,0,errors.New("Redirect")}
			rn.commitChan <- ci
		case "Commit":
			//? for all or just leader
			ci := CommitInfo{e.Data["data"].([]byte),e.Data["index"].(uint64),nil}
			rn.commitIndex = ci.Index
			rn.commitChan <- ci

		}

	}
}

func (rn *RaftNode) processActions() {
	//? Stop using process channel
	for {
		rn.processAction()
	}
}

func (rn *RaftNode) alarm(d time.Duration) {
	rn.timer.Reset(d)	
}

func (rn *RaftNode) Start() {
	go rn.processActions()
	go rn.processInboxEvents()
	rn.sm.Start()
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
	rn.id = conf.Id
	//rn.leaderId = 0    //? 0 is invalid
	rn.commitIndex = 0 //?verify
	rnlog, err := log.Open(conf.LogDir)
	if err != nil {
fmt.Println("DP15",err)
		return nil	//errror
	}
	rn.rnlog = rnlog
	rn.rnlog.RegisterSampleEntry(raftsm.LogEntry{})

	rn.server, err = GetServer(int(conf.Id), conf.Cluster)
	if err != nil {
fmt.Println("DP16",err)
		return nil	//error
	}


	//--Init State Machine ---
	peerIds := make([]uint64, 0)
	for _, e := range conf.Cluster {
		if e.Id != conf.Id {
			peerIds = append(peerIds,e.Id)
		}
	}
	rn.majority = uint64(math.Ceil(float64(len(conf.Cluster)) / 2.0)) //? Assumes len is always odd
	rn.statefile = conf.LogDir+"/TermVotedFor"                                        //? change required
	data, err := ioutil.ReadFile(rn.statefile)
	if err != nil {
fmt.Println("DP17",err)
		
			tvf := TermVotedFor{0,0}
			data, err := json.Marshal(tvf)
			if err != nil {
fmt.Println("DP19",err)
				//return err//error
			}
			err = ioutil.WriteFile(rn.statefile, data, 0777)
			if err != nil {

fmt.Println("DP20",err)
				//return err//error
			}

		rn.currentTerm = 0
		rn.votedFor = 0
	} else {
		var tvf TermVotedFor
		err = json.Unmarshal(data, &tvf)
		if err != nil {
fmt.Println("DP18",err)
			return nil //error
		}
		rn.currentTerm = tvf.CurrentTerm
		rn.votedFor = tvf.VotedFor
	}

	rn.logIndex = uint64(rn.rnlog.GetLastIndex())
	le := make([]raftsm.LogEntry,1) //? initilize from Log
	le[0] = raftsm.LogEntry{0,nil,false}

	if(rn.rnlog.GetLastIndex()!=int64(-1)) {
		for i:=uint64(0);i<=rn.logIndex;i++ {
			tmp,err := rn.rnlog.Get(int64(i))
			if err==nil {
				le = append(le,tmp.(raftsm.LogEntry))
			} else {
				return nil //error
			}
		}
	}
fmt.Println("DP14 LOG:",le)
	rn.sm = raftsm.InitStateMachine(conf.Id, peerIds, rn.majority, time.Duration(conf.ElectionTimeout)*time.Millisecond, time.Duration(conf.HeartbeatTimeout)*time.Millisecond, rn.currentTerm, rn.votedFor /*//?*/, le)

	rn.smActionChan = *rn.sm.GetActionChannel()
	rn.smEventChan = *rn.sm.GetEventChannel()
	rn.commitChan = make(chan CommitInfo, 100)



	
	rn.timer = time.AfterFunc(100000*time.Second, func(){
				rn.smEventChan <- raftsm.CreateEvent("Timeout")
			})
	rn.timer.Stop()
	gob.Register(raftsm.Event{})
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
	return nil, (data.(raftsm.LogEntry)).Data
}

func (rn *RaftNode) Id() uint64 {
	return rn.id
}

func (rn *RaftNode) LeaderId() uint64 {
	return rn.sm.GetLeaderId()
}

func (rn *RaftNode) Shutdown() {
	rn.server.Close()
	rn.rnlog.Close()
	rn.sm.Stop()
	//Stop the machine
}
