package raft

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/log"
	"io/ioutil"
	"math"
	"strconv"
	"time"
)

func ResereveImport2() {
	fmt.Println("")
}

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
	logIndex  uint64
	statefile string
	timer     *time.Timer

	rnlog      *log.Log
	server     cluster.Server
	commitChan chan CommitInfo

	sm *StateMachine

	smAlarmChan  chan Action
	smCommitChan chan Action
	smSaveChan   chan Action
	smSendChan   chan Action

	smTimeoutChan  chan Event
	smResponseChan chan Event
	smRequestChan  chan Event

	stopChan chan int
}

func (rn *RaftNode) processInboxEvent() {
	select {
	case e := <-rn.server.Inbox():
		switch e.Msg.(Event).Name {
		case "Append":
			rn.smRequestChan <- e.Msg.(Event)
		case "Timeout":
			rn.smTimeoutChan <- e.Msg.(Event)
		case "AppendEntriesReq":
			rn.smRequestChan <- e.Msg.(Event)
		case "AppendEntriesResp":
			rn.smResponseChan <- e.Msg.(Event)
		case "VoteReq":
			rn.smRequestChan <- e.Msg.(Event)
		case "VoteResp":
			rn.smResponseChan <- e.Msg.(Event)
		}
	}
}

func (rn *RaftNode) processInboxEvents() {
	for {
		select {
		case <-rn.stopChan:
			return
		default:
		}
		rn.processInboxEvent()
	}
}

func (rn *RaftNode) getPrioritizedAction() Action {
	e := Action{}
	select {
	case e = <-rn.smAlarmChan:
	default:
		select {
		case e = <-rn.smAlarmChan:
		case e = <-rn.smCommitChan:
		default:
			select {
			case e = <-rn.smAlarmChan:
			case e = <-rn.smCommitChan:
			case e = <-rn.smSaveChan:
			default:
				select {
				case e = <-rn.smAlarmChan:
				case e = <-rn.smCommitChan:
				case e = <-rn.smSaveChan:
				case e = <-rn.smSendChan:
				}
			}
		}
	}
	return e
}

func (rn *RaftNode) processAction() {
	e := rn.getPrioritizedAction()
	//fmt.Println(rn.id,":",e,"time:",time.Now())
	switch e.Name {
	case "LogStore":
		/* Done in StateMachine
		l := LogEntry{e.Data["term"].(uint64), e.Data["data"].([]byte), true}
		idx := int64(e.Data["index"].(uint64)) - 1
		if idx <= rn.rnlog.GetLastIndex() + 1 {
			rn.rnlog.TruncateToEnd(idx)
		}
		err := rn.rnlog.Append(l)
		if err != nil {
			//error
		}
		*/
	case "SaveState":
		tvf := TermVotedFor{e.Data["currentTerm"].(uint64), e.Data["votedFor"].(uint64)}
		data, err := json.Marshal(tvf)
		if err != nil {
			//return nil,err//error
		}
		err = ioutil.WriteFile(rn.statefile, data, 0777)
		if err != nil {
			//fmt.Println("File write errorr", err)
			//return nil,err//error
		}
	case "Alarm":
		rn.alarm(e.Data["t"].(time.Duration))
	case "Send":
		id := int(e.Data["peerId"].(uint64))
		rn.server.Outbox() <- &cluster.Envelope{Pid: id, Msg: e.Data["event"].(Event)}
	case "Redirect":
		ci := CommitInfo{e.Data["data"].([]byte), 0, errors.New("Redirect")}
		rn.commitChan <- ci
	case "Commit":
		//? for all or just leader
		ci := CommitInfo{e.Data["data"].([]byte), e.Data["index"].(uint64), nil}
		rn.commitIndex = ci.Index
		rn.commitChan <- ci

	}
}

func (rn *RaftNode) processActions() {
	//? Stop using process channel
	for {
		select {
		case <-rn.stopChan:
			return
		default:
		}
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

func CreateRaftNode(conf Config) (*RaftNode, error) {
	rn := new(RaftNode)
	rn.id = conf.Id
	//rn.leaderId = 0    //? 0 is invalid
	rn.commitIndex = 0 //?verify
	rnlog, err := log.Open(conf.LogDir)
	if err != nil {
		return nil, err
	}
	rn.rnlog = rnlog
	rn.rnlog.RegisterSampleEntry(LogEntry{})

	if rn.rnlog.GetLastIndex() == -1 {
		rn.rnlog.Append(LogEntry{0, nil, false})
	}

	rn.server, err = GetServer(int(conf.Id), conf.Cluster)
	if err != nil {
		return nil, err
	}

	//--Init State Machine ---
	peerIds := make([]uint64, 0)
	for _, e := range conf.Cluster {
		if e.Id != conf.Id {
			peerIds = append(peerIds, e.Id)
		}
	}
	rn.majority = uint64(math.Ceil(float64(len(conf.Cluster)) / 2.0)) //? Assumes len is always odd
	rn.statefile = conf.LogDir + "/TermVotedFor"                      //? change required
	data, err := ioutil.ReadFile(rn.statefile)
	if err != nil {
		tvf := TermVotedFor{0, 0}
		data, err := json.Marshal(tvf)
		if err != nil {
			return nil, err
		}
		err = ioutil.WriteFile(rn.statefile, data, 0777)
		if err != nil {
			return nil, err
		}

		rn.currentTerm = 0
		rn.votedFor = 0
	} else {
		var tvf TermVotedFor
		err = json.Unmarshal(data, &tvf)
		if err != nil {
			return nil, err
		}
		rn.currentTerm = tvf.CurrentTerm
		rn.votedFor = tvf.VotedFor
	}

	rn.logIndex = uint64(rn.rnlog.GetLastIndex()) + 1

	rn.sm = InitStateMachine(conf.Id, peerIds, rn.majority, time.Duration(conf.ElectionTimeout)*time.Millisecond, time.Duration(conf.HeartbeatTimeout)*time.Millisecond, rn.currentTerm, rn.votedFor /*//?*/, rn.rnlog)

	rn.smAlarmChan = *rn.sm.GetAlarmChannel()
	rn.smCommitChan = *rn.sm.GetCommitChannel()
	rn.smSaveChan = *rn.sm.GetSaveChannel()
	rn.smSendChan = *rn.sm.GetSendChannel()

	rn.smTimeoutChan = *rn.sm.GetTimeoutChannel()
	rn.smResponseChan = *rn.sm.GetResponseChannel()
	rn.smRequestChan = *rn.sm.GetRequestChannel()

	rn.commitChan = make(chan CommitInfo, 1000)
	rn.stopChan = make(chan int, 2)

	rn.timer = time.AfterFunc(100000*time.Second, func() {
		//fmt.Println(rn.id,": timeout at:",time.Now())
		rn.smTimeoutChan <- CreateEvent("Timeout")
	})
	rn.timer.Stop()
	gob.Register(Event{})
	return rn, nil
}

func (rn *RaftNode) Append(data []byte) {
	rn.smRequestChan <- CreateEvent("Append", "data", data)
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
	return nil, (data.(LogEntry)).Data
}

func (rn *RaftNode) Id() uint64 {
	return rn.id
}

func (rn *RaftNode) LeaderId() uint64 {
	return rn.sm.GetLeaderId()
}

func (rn *RaftNode) Shutdown() {
	rn.stopChan <- 1
	rn.stopChan <- 2
	rn.sm.Stop()
	rn.server.Close()
	rn.rnlog.Close()
}
