package raft

import (
	"fmt"
	"github.com/cs733-iitb/log"
	"reflect"
	"testing"
	"time"
)

const (
	ELECTION_TIMEOUT  = 1000 * time.Millisecond
	HEARTBEAT_TIMEOUT = 50 * time.Millisecond
)

//----------------SUPPORTERS-----------

func GetLog(dir string) *log.Log {
	rnlog, err := log.Open(dir)
	if err != nil {
		fmt.Println("Error greating log :", err)
		return nil
	}
	rnlog.RegisterSampleEntry(LogEntry{})
	rnlog.TruncateToEnd(0)
	if rnlog.GetLastIndex() == -1 {
		rnlog.Append(LogEntry{0, nil, false})
	}
	return rnlog
}

func GetFollowerSM() *StateMachine {
	sm := InitStateMachine(uint64(10), []uint64{20, 30, 40, 50}, uint64(3), time.Duration(ELECTION_TIMEOUT)*time.Millisecond, time.Duration(HEARTBEAT_TIMEOUT)*time.Millisecond, uint64(1), uint64(0), GetLog("FollowerLog"))
	return sm
}

func GetCandidateSM() *StateMachine {
	sm := InitStateMachine(uint64(10), []uint64{20, 30, 40, 50}, uint64(3), time.Duration(ELECTION_TIMEOUT)*time.Millisecond, time.Duration(HEARTBEAT_TIMEOUT)*time.Millisecond, uint64(2), uint64(10), GetLog("CandidateLog"))
	sm.state = CANDIDATE
	return sm
}

func GetLeaderSM() *StateMachine {
	sm := InitStateMachine(uint64(10), []uint64{20, 30, 40, 50}, uint64(3), time.Duration(ELECTION_TIMEOUT)*time.Millisecond, time.Duration(HEARTBEAT_TIMEOUT)*time.Millisecond, uint64(2), uint64(10), GetLog("LeaderLog"))
	sm.state = LEADER

	for i := range sm.nextIndex {
		sm.nextIndex[i] = sm.logIndex
		sm.matchIndex[i] = 0
	}

	return sm
}

func getPrioritizedAction(sm *StateMachine) Action {
	e := Action{}
	select {
	case e = <-sm.alarmChan:
	default:
		select {
		case e = <-sm.alarmChan:
		case e = <-sm.commitChan:
		default:
			select {
			case e = <-sm.alarmChan:
			case e = <-sm.commitChan:
			case e = <-sm.saveChan:
			default:
				select {
				case e = <-sm.alarmChan:
				case e = <-sm.commitChan:
				case e = <-sm.saveChan:
				case e = <-sm.sendChan:
				case <-time.After(10 * time.Millisecond):
				}
			}
		}
	}
	return e
}

func CheckActions(t *testing.T, sm *StateMachine, testname string, actions []string, count []int) {
	counter := make([]int, len(actions))
	for {
		a := getPrioritizedAction(sm)
		if reflect.DeepEqual(a, Action{}) {
			break
		}

		//fmt.Println(a)
		invalidaction := true
		for i := range actions {
			if a.Name == actions[i] {
				counter[i]++
				invalidaction = false
				break
			}
		}
		if invalidaction {
			t.Error(testname, "Invalid Action :", a)
		}
	}
	for i := range count {
		if counter[i] != count[i] {
			t.Error(testname, actions[i], "Count mismatch    required:", count[i], "got:", counter[i])
		}
	}
}

//--------TestStartStop---------
//This testcase results in race condition for sm.stop variable but it is not a problem whoever wins the race
//Commenting this testcase will pass the go test -race

func TestStartStop(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	//Try to stop before start
	sm.Stop()
	//Try multiple starts. Only first should succeed
	//fmt.Println(sm.Start(),time.Now())
	if sm.Start() != nil {
		t.Error("Valid start failed\n")
	}
	if sm.Start() == nil {
		t.Error("Invalid start succeeded\n")
	}
	if sm.Start() == nil {
		t.Error("Invalid start succeeded\n")
	}
	//Stop and then start
	sm.Stop()

	if sm.Start() != nil {
		t.Error("Valid start failed\n")
	}
	//Try Multiple stop and then multiple start. Only first should succeed.
	sm.Stop()
	sm.Stop()
	sm.Stop()
	if sm.Start() != nil {
		t.Error("Valid start failed\n")
	}
	if sm.Start() == nil {
		t.Error("Invalid start succeeded\n")
	}
	if sm.Start() == nil {
		t.Error("Invalid start succeeded\n")
	}
	sm.Stop()

	sm.Start()
	//check for Alarm action at start()
	ta := CreateAction("Alarm", "t", time.Duration(ELECTION_TIMEOUT)*time.Millisecond) //test action
	a, ok := <-sm.alarmChan
	if !ok || !reflect.DeepEqual(a, ta) {
		t.Error("No alarm event at start\n")
	}
	sm.Stop()
}

func TestFollowerTimeout(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	term := sm.currentTerm
	sm.timeoutChan <- CreateEvent("Timeout")
	sm.processEvent()
	//SM should move to candidate state
	if sm.state != CANDIDATE {
		t.Error("Invalid mode\n")
	}
	//Current term must be incremented and saved
	if sm.currentTerm != term+1 {
		t.Error("Mismatch in currentTerm\n")
	}
	//ask for votes and set election timeout
	//Alarm for electionTimeout : 1
	//Send votes to all peers : number of peers
	//SaveState of currentTerm and votedFor : 1
	CheckActions(t, sm, "TestFollowerTimeout", []string{"Alarm", "Send", "SaveState"}, []int{1, len(sm.peers), 1})
}

func TestCandidateTimeout(t *testing.T) { //??Improvement required for TestCandidateTimeout
	sm := GetCandidateSM()
	defer sm.log.Close() //SM in candidate mode and vote requests sent
	sm.timeoutChan <- CreateEvent("Timeout")
	sm.processEvent()
	if sm.state != FOLLOWER {
		t.Error("Not able to move to follower state")
	}
	CheckActions(t, sm, "TestCandidateTimeout", []string{"SaveState", "Alarm"}, []int{1, 1})

}

func TestPositiveVoteRespToCandidate(t *testing.T) {
	sm := GetCandidateSM()
	defer sm.log.Close() //SM in candidate mode and vote requests sent
	event := CreateEvent("VoteResp", "term", sm.currentTerm, "voteGranted", true)
	for i := uint64(1); i < sm.majority; i++ {
		sm.responseChan <- event
		sm.processEvent()
		//Shoud not move to Leader state
		if sm.state != CANDIDATE {
			t.Error("Bad state after ", i, " +ve VoteResp")
		}
	}
	sm.responseChan <- event
	sm.processEvent()
	//Shoud move to Leader state and send heartbeats to all and reset alarm
	if sm.state != LEADER {
		t.Error("Not able to move to leader state")
	}
	CheckActions(t, sm, "TestPositiveVoteRespToCandidate", []string{"Alarm", "Send"}, []int{1, len(sm.peers)})
}

func TestNegativeVoteRespWithGreaterTerm(t *testing.T) {
	sm := GetCandidateSM()
	defer sm.log.Close()   //SM in candidate mode and vote requests sent
	term := sm.currentTerm //save it for comparison later
	event := CreateEvent("VoteResp", "term", sm.currentTerm+1, "voteGranted", false)
	sm.responseChan <- event
	sm.processEvent()
	//Shoud update term and move to Follower state and wait for AppendEntriesReq or Timeout
	if sm.state != FOLLOWER {
		t.Error("Not able to move to follower state")
	}
	if sm.currentTerm != term+1 {
		t.Error("Term not updated")
	}
	CheckActions(t, sm, "TestNegativeVoteRespWithGreaterTerm", []string{"Alarm", "SaveState"}, []int{1, 1})
}

func TestNegativeVoteRespWithSameTerm(t *testing.T) {
	sm := GetCandidateSM()
	defer sm.log.Close()
	event := CreateEvent("VoteResp", "term", sm.currentTerm, "voteGranted", false)
	sm.responseChan <- event
	sm.processEvent()
	//Shoud remain in same state
	if sm.state != CANDIDATE {
		t.Error("State error")
	}
	//There should be no actions
	CheckActions(t, sm, "TestNegativeVoteRespWithSameTerm", []string{}, []int{})
}

func TestVoteReqToFollowerWithSameTerm(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	term := sm.currentTerm //save term for later check
	//Vote request with same term and longer log
	sm.requestChan <- CreateEvent("VoteReq", "term", sm.currentTerm, "candidateId", uint64(50), "lastLogIndex", sm.logIndex+1, "lastLogTerm", sm.currentTerm-1)
	sm.processEvent()
	//Must remain in follower mode
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Current term must remain same
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	//Cast vote and save votedFor
	CheckActions(t, sm, "TestVoteReqToFollowerWithSameTerm", []string{"Send", "SaveState"}, []int{1, 1})
}

func TestVoteReqToFollowerWithSameTermSameLog(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	term := sm.currentTerm //save term for later check
	//Vote request with same term and same log
	sm.requestChan <- CreateEvent("VoteReq", "term", sm.currentTerm, "candidateId", uint64(50), "lastLogIndex", sm.logIndex-1, "lastLogTerm", sm.logElementAt(sm.logIndex-1).Term)
	sm.processEvent()
	//Must remain in follower mode
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Current term must remain same
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	//Cast vote and save votedFor
	CheckActions(t, sm, "TestVoteReqToFollowerWithSameTermSameLog", []string{"Send", "SaveState"}, []int{1, 1})
}

func TestVoteReqToFollowerWithHigherTerm(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	term := sm.currentTerm //save term for later check
	//Vote request with higher term but smaller log
	sm.requestChan <- CreateEvent("VoteReq", "term", sm.currentTerm+2, "candidateId", uint64(50), "lastLogIndex", sm.logIndex-2, "lastLogTerm", sm.currentTerm+1)
	sm.processEvent()
	//Must remain in follower mode
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Current term must remain same
	if sm.currentTerm != term+2 {
		t.Error("Mismatch in currentTerm\n")
	}
	//Cast vote and save votedFor
	CheckActions(t, sm, "TestVoteReqToFollowerWithHigherTerm", []string{"Send", "SaveState"}, []int{1, 2})
}

func TestVoteReqSentToAlreadyVotedFollower(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	sm.votedFor = uint64(60)
	term := sm.currentTerm //save term for later check
	//VoteRequest with same term
	sm.requestChan <- CreateEvent("VoteReq", "term", sm.currentTerm, "candidateId", uint64(50), "lastLogIndex", sm.logIndex+1, "lastLogTerm", sm.currentTerm-1)
	sm.processEvent()
	//Must remain in follower mode
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Current term must remain same
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	//Cast negetive vote
	CheckActions(t, sm, "TestVoteReqSentToAlreadyVotedFollower", []string{"Send"}, []int{1})

}

func TestVoteReqWithHigherTermSentToAlreadyVotedFollower(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	sm.votedFor = uint64(60)
	term := sm.currentTerm //save term for later check
	//VoteRequest with higher term
	sm.requestChan <- CreateEvent("VoteReq", "term", sm.currentTerm+2, "candidateId", uint64(50), "lastLogIndex", sm.logIndex+1, "lastLogTerm", sm.currentTerm-1)
	sm.processEvent()
	//Must remain in follower mode
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Current term must remain same
	if sm.currentTerm != term+2 {
		t.Error("Mismatch in currentTerm\n")
	}
	//Cast vote and save term and votedfor
	CheckActions(t, sm, "TestVoteReqWithHigherTermSentToAlreadyVotedFollower", []string{"Send", "SaveState"}, []int{1, 2})
}

func TestVoteReqToLeaderWithHigherTerm(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	term := sm.currentTerm //save term for later check
	//Vote request with same term and longer log
	sm.requestChan <- CreateEvent("VoteReq", "term", sm.currentTerm+2, "candidateId", uint64(50), "lastLogIndex", sm.logIndex+1, "lastLogTerm", sm.currentTerm+1)
	sm.processEvent()
	//Must move to follower mode
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Current term must be updated
	if sm.currentTerm != term+2 {
		t.Error("Mismatch in currentTerm\n")
	}
	//Cast vote and save votedFor
	CheckActions(t, sm, "TestVoteReqToLeaderWithHigherTerm", []string{"Send", "SaveState", "Alarm"}, []int{1, 1, 1})
}

func TestVoteReqToLeaderWithLesserTerm(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	term := sm.currentTerm //save term for later check
	//Vote request with same term and longer log
	sm.requestChan <- CreateEvent("VoteReq", "term", sm.currentTerm-1, "candidateId", uint64(50), "lastLogIndex", sm.logIndex+1, "lastLogTerm", sm.currentTerm-2)
	sm.processEvent()
	//Must retain leader mode
	if sm.state != LEADER {
		t.Error("Invalid mode\n")
	}
	//Current term must be same
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	//Send -ve resp
	CheckActions(t, sm, "TestVoteReqToLeaderWithLesserTerm", []string{"Send"}, []int{1})
}

func TestVoteReqToCandidateWithHigherTerm(t *testing.T) {
	sm := GetCandidateSM()
	defer sm.log.Close()
	term := sm.currentTerm //save term for later check
	//Vote request with same term and longer log
	sm.requestChan <- CreateEvent("VoteReq", "term", sm.currentTerm+2, "candidateId", uint64(50), "lastLogIndex", sm.logIndex+1, "lastLogTerm", sm.currentTerm+1)
	sm.processEvent()
	//Must move to follower mode
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Current term must be updated
	if sm.currentTerm != term+2 {
		t.Error("Mismatch in currentTerm\n")
	}
	//Cast vote and save votedFor
	CheckActions(t, sm, "TestVoteReqToLeaderWithHigherTerm", []string{"Send", "SaveState", "Alarm"}, []int{1, 1, 1})
}

func TestVoteReqToCandidateWithLesserTerm(t *testing.T) {
	sm := GetCandidateSM()
	defer sm.log.Close()
	term := sm.currentTerm //save term for later check
	//Vote request with same term and longer log
	sm.requestChan <- CreateEvent("VoteReq", "term", sm.currentTerm-1, "candidateId", uint64(50), "lastLogIndex", sm.logIndex+1, "lastLogTerm", sm.currentTerm-2)
	sm.processEvent()
	//Must retain leader mode
	if sm.state != CANDIDATE {
		t.Error("Invalid mode\n")
	}
	//Current term must be same
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	//Send -ve resp
	CheckActions(t, sm, "TestVoteReqToLeaderWithLesserTerm", []string{"Send"}, []int{1})
}

func TestLeaderTimeout(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	term := sm.currentTerm
	sm.timeoutChan <- CreateEvent("Timeout")
	sm.processEvent()
	//Must retain leader state
	if sm.state != LEADER {
		t.Error("Invalid mode\n")
	}
	//Must retain same term
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	//Alarm for heartbeatTimeout : 1
	//Send heartbeats to all peers : number of peers
	CheckActions(t, sm, "TestLeaderTimeout", []string{"Alarm", "Send"}, []int{1, len(sm.peers)})
}

func TestAppendToFollower(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	//Get channels
	//save for later use
	term := sm.currentTerm
	logIndex := sm.logIndex
	commitIndex := sm.commitIndex

	sm.requestChan <- CreateEvent("Append", "data", []byte{0, 255, 10, 7, 13, 1, 2})
	sm.processEvent()
	//Must retain leader state
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Must retain same term
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	//Logindex check
	if sm.logIndex != logIndex {
		t.Error("Invalid log index\n")
	}
	//Commit index check
	if sm.commitIndex != commitIndex {
		t.Error("Invalid commit index\n")
	}

	//Redirect
	CheckActions(t, sm, "TestAppendToFollower", []string{"Redirect"}, []int{1})
}

func TestAppendToLeader(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	term := sm.currentTerm
	logIndex := sm.logIndex
	commitIndex := sm.commitIndex

	sm.requestChan <- CreateEvent("Append", "data", []byte{})
	sm.processEvent()
	//Must retain leader state
	if sm.state != LEADER {
		t.Error("Invalid mode\n")
	}
	//Must retain same term
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	//Logindex check
	if sm.logIndex != logIndex+1 {
		t.Error("Invalid log index\n")
	}
	//Commit index check
	if sm.commitIndex != commitIndex {
		t.Error("Invalid commit index\n")
	}
	//Reset alarm for heartbeatTimeout : 1
	//Send AppendEntriesReq to all peers : number of peers
	//Send LogStore to layer above
	CheckActions(t, sm, "TestAppendToLeader", []string{"Alarm", "Send", "LogStore"}, []int{1, len(sm.peers), 1})
}

func TestPossitiveAppendEntriesResp(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	//Setup Leader
	sm.log.Append(LogEntry{sm.currentTerm, []byte{0, 255, 10, 7, 13, 1, 2}, true})
	sm.logIndex++
	for i := range sm.nextIndex {
		sm.nextIndex[i] = sm.logIndex - 1
		sm.matchIndex[i] = sm.logIndex - 2
	}
	//Get channels
	//save for later use
	commitIndex := sm.commitIndex

	//Positive responses one less than majority
	for i := uint64(0); i < sm.majority-2; i++ {
		sm.responseChan <- CreateEvent("AppendEntriesResp", "term", sm.currentTerm, "success", true, "senderId", sm.peers[i], "forIndex", sm.logIndex-1)
		sm.processEvent()
	}
	//Commit index must be same
	if sm.commitIndex != commitIndex {
		t.Error("Invalid commit index\n")
	}
	//Next and match index must be updated for < majority peers
	for i := uint64(0); i < sm.majority-2; i++ {
		if sm.nextIndex[i] != sm.logIndex || sm.matchIndex[i] != sm.logIndex-1 {
			t.Error("Next or Match index mismatch for index", i)
		}
	}
	//Next and match must not be updated for >= majority peers
	for i := sm.majority - 2; i < uint64(len(sm.peers)); i++ {
		if sm.nextIndex[i] != sm.logIndex-1 || sm.matchIndex[i] != sm.logIndex-2 {
			t.Error("Next or Match index mismatch for index", i)
		}
	}
	//No action must appear till now
	CheckActions(t, sm, "PossitiveAppendEntriesResp 1", []string{}, []int{})

	//Now majority +ve responses
	sm.responseChan <- CreateEvent("AppendEntriesResp", "term", sm.currentTerm, "success", true, "senderId", sm.peers[sm.majority-2], "forIndex", sm.logIndex-1)
	sm.processEvent()

	//Commit index must be incremented
	if sm.commitIndex != commitIndex+1 {
		t.Error("Invalid commit index\n")
	}
	//Next and match index must be updated for <= majority peers
	for i := uint64(0); i < sm.majority-1; i++ {
		if sm.nextIndex[i] != sm.logIndex || sm.matchIndex[i] != sm.logIndex-1 {
			t.Error("Next or Match index mismatch for index", i, sm.nextIndex[i], sm.logIndex, sm.matchIndex[i])
		}
	}
	//Next and match must not be updated for > majority peers
	for i := sm.majority - 1; i < uint64(len(sm.peers)); i++ {
		if sm.nextIndex[i] != sm.logIndex-1 || sm.matchIndex[i] != sm.logIndex-2 {
			t.Error("Next or Match index mismatch for index", i)
		}
	}
	CheckActions(t, sm, "PossitiveAppendEntriesResp 2", []string{"Commit"}, []int{1})
}

func TestHeartbeatResp(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	//Get channels
	term := sm.currentTerm
	sm.responseChan <- CreateEvent("AppendEntriesResp", "term", sm.currentTerm, "success", true, "senderId", sm.peers[0], "forIndex", uint64(0))
	sm.processEvent()
	if sm.state != LEADER {
		t.Error("Invalid mode\n")
	}
	//Must retain same term
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	//Do nothing
	CheckActions(t, sm, "TestHeartbeatResp", []string{}, []int{})
}

func TestBackingFollowerPositiveAppendEntriesResp(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	//Setup Leader
	sm.log.Append(LogEntry{sm.currentTerm, []byte{0, 255, 10, 7, 13, 1, 2}, true})
	sm.log.Append(LogEntry{sm.currentTerm, []byte{0, 255, 10, 7, 13, 1, 2}, true})
	sm.logIndex += 2
	sm.nextIndex[0] = sm.logIndex - 2
	//Get channels
	//save for later use
	commitIndex := sm.commitIndex

	sm.responseChan <- CreateEvent("AppendEntriesResp", "term", sm.currentTerm, "success", true, "senderId", sm.peers[0], "forIndex", sm.logIndex-1)
	sm.processEvent()

	//Commit index must be same
	if sm.commitIndex != commitIndex {
		t.Error("Invalid commit index\n")
	}
	//Next index of peer[0] must be incremented and leader must send next log entry
	if sm.nextIndex[0] != sm.logIndex-1 {
		t.Error("Next index mismatch\n")
	}
	CheckActions(t, sm, "TestBackingFollowerPositiveAppendEntriesResp", []string{"Send"}, []int{1})
}

func TestBackingFollowerNegativeAppendEntriesResp(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	//Setup Leader
	sm.log.Append(LogEntry{sm.currentTerm, []byte{0, 255, 10, 7, 13, 1, 2}, true})
	sm.log.Append(LogEntry{sm.currentTerm, []byte{0, 255, 10, 7, 13, 1, 2}, true})
	sm.logIndex += 2
	for i := range sm.nextIndex {
		sm.nextIndex[i] = sm.logIndex - 1
	}
	//Get channels
	//save for later use
	commitIndex := sm.commitIndex

	sm.responseChan <- CreateEvent("AppendEntriesResp", "term", sm.currentTerm, "success", false, "senderId", sm.peers[0], "forIndex", sm.logIndex-1)
	sm.processEvent()

	//Commit index must be same
	if sm.commitIndex != commitIndex {
		t.Error("Invalid commit index\n")
	}
	//Next index of peer[0] must be decremented and leader must send previous log entry
	if sm.nextIndex[0] != sm.logIndex-2 {
		t.Error("Next index mismatch\n")
	}
	CheckActions(t, sm, "TestBackingFollowerNegativeAppendEntriesResp", []string{"Send"}, []int{1})
}

func TestNegativeAppendEntriesRespWithHigherTerm(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	//Get channels
	//save for later use
	term := sm.currentTerm

	sm.responseChan <- CreateEvent("AppendEntriesResp", "term", sm.currentTerm+1, "success", false, "senderId", sm.peers[0], "forIndex", sm.logIndex-1)
	sm.processEvent()

	//Must change to follower state
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Must update term
	if sm.currentTerm != term+1 {
		t.Error("Mismatch in currentTerm\n")
	}
	//Must reset voted for
	if sm.votedFor != 0 {
		t.Error("Mismatch in currentTerm\n")
	}
	CheckActions(t, sm, "TestNegativeAppendEntriesRespWithHigherTerm", []string{"Alarm", "SaveState"}, []int{1, 1})
}

func TestAppendEntriesReqToLeaderWithHigherTerm(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	//Get channels
	//save for later use
	term := sm.currentTerm
	sm.requestChan <- CreateEvent("AppendEntriesReq", "term", sm.currentTerm+2, "leaderId", sm.peers[0], "prevLogIndex", uint64(0), "prevLogTerm", sm.logElementAt(0).Term, "entries", LogEntry{0, []byte{}, false}, "leaderCommit", uint64(0))
	sm.processEvent()
	//Must change to follower state
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Must update term
	if sm.currentTerm != term+2 {
		t.Error("Mismatch in currentTerm\n")
	}
	//Must reset voted for
	if sm.votedFor != sm.peers[0] {
		t.Error("Mismatch in currentTerm\n")
	}

	CheckActions(t, sm, "TestAppendEntriesReqToLeaderWithHigherTerm", []string{"Alarm", "SaveState", "Send"}, []int{1, 1, 1})
}

func TestAppendEntriesReqToFollower(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	sm.log.Append(LogEntry{1, []byte{1, 2, 3}, true})
	//Get channels
	//save for later use
	term := sm.currentTerm
	sm.requestChan <- CreateEvent("AppendEntriesReq", "term", sm.currentTerm+2, "leaderId", sm.peers[0], "prevLogIndex", uint64(0), "prevLogTerm", sm.logElementAt(0).Term, "entries", LogEntry{0, []byte{}, true}, "leaderCommit", uint64(1))
	sm.processEvent()
	//Must remain in follower state
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Must update term
	if sm.currentTerm != term+2 {
		t.Error("Mismatch in currentTerm\n")
	}
	//Must reset voted for
	if sm.votedFor != sm.peers[0] {
		t.Error("Mismatch in currentTerm\n")
	}

	CheckActions(t, sm, "TestAppendEntriesReqToFollower", []string{"Alarm", "SaveState", "Send", "LogStore", "Commit"}, []int{1, 1, 1, 1, 1})
}

func TestAppendEntriesReqToFollowerWithHigherCommit(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	sm.log.Append(LogEntry{1, []byte{1, 2, 3}, true})
	sm.commitIndex = uint64(0)
	term := sm.currentTerm
	sm.requestChan <- CreateEvent("AppendEntriesReq", "term", sm.currentTerm, "leaderId", sm.peers[0], "prevLogIndex", uint64(0), "prevLogTerm", sm.logElementAt(0).Term, "entries", LogEntry{0, []byte{}, true}, "leaderCommit", sm.logIndex+5)
	sm.processEvent()
	//Must change to follower state
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Must update term
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	if sm.commitIndex != sm.logIndex-1 {
		t.Error("Commit index mismatch")
	}

	CheckActions(t, sm, "TestAppendEntriesReqToFollowerWithHigherCommit", []string{"Alarm", "Send", "LogStore", "Commit"}, []int{1, 1, 1, 1})
}

func TestAppendEntriesReqToFollowerWithLargerPrevIndex(t *testing.T) {
	sm := GetFollowerSM()
	defer sm.log.Close()
	sm.log.Append(LogEntry{1, []byte{1, 2, 3}, true})
	sm.commitIndex = uint64(0)
	term := sm.currentTerm
	sm.requestChan <- CreateEvent("AppendEntriesReq", "term", sm.currentTerm, "leaderId", sm.peers[0], "prevLogIndex", uint64(sm.log.GetLastIndex()+6), "prevLogTerm", sm.logElementAt(0).Term, "entries", LogEntry{0, []byte{}, true}, "leaderCommit", sm.logIndex)
	sm.processEvent()
	//Must change to follower state
	if sm.state != FOLLOWER {
		t.Error("Invalid mode\n")
	}
	//Must update term
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}

	CheckActions(t, sm, "TestAppendEntriesReqToFollowerWithLargerPrevIndex", []string{"Alarm", "Send"}, []int{1, 1})
}

func TestAppendEntriesReqToLeaderWithLesserTerm(t *testing.T) {
	sm := GetLeaderSM()
	defer sm.log.Close()
	//Get channels
	//save for later use
	term := sm.currentTerm
	sm.requestChan <- CreateEvent("AppendEntriesReq", "term", sm.currentTerm-1, "leaderId", sm.peers[0], "prevLogIndex", uint64(1), "prevLogTerm", sm.currentTerm, "entries", LogEntry{0, []byte{}, false}, "leaderCommit", uint64(1))
	sm.processEvent()
	//Must not change state
	if sm.state != LEADER {
		t.Error("Invalid mode\n")
	}
	//Same term
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	CheckActions(t, sm, "TestAppendEntriesReqToLeaderWithLesserTerm", []string{"Send"}, []int{1})
}

func TestAppendEntriesReqToCandidateWithLesserTerm(t *testing.T) {
	sm := GetCandidateSM()
	defer sm.log.Close()
	//Get channels
	//save for later use
	term := sm.currentTerm
	sm.requestChan <- CreateEvent("AppendEntriesReq", "term", sm.currentTerm-1, "leaderId", sm.peers[0], "prevLogIndex", uint64(1), "prevLogTerm", sm.currentTerm, "entries", LogEntry{0, []byte{}, false}, "leaderCommit", uint64(1))
	sm.processEvent()
	//Must not change state
	if sm.state != CANDIDATE {
		t.Error("Invalid mode\n")
	}
	//Same term
	if sm.currentTerm != term {
		t.Error("Mismatch in currentTerm\n")
	}
	CheckActions(t, sm, "TestAppendEntriesReqToCandidateWithLesserTerm", []string{"Send"}, []int{1})
}
