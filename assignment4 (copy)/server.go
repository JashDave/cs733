package fileserver

import (
	"bufio"
	"fmt"
	"github.com/JashDave/cs733/assignment4/fs"
	raft "github.com/JashDave/cs733/assignment4/assignment3"
	"net"
	"os"
	"strconv"
	"encoding/json"
)

var crlf = []byte{'\r', '\n'}

type ClientHandeler struct {
	rn *raft.RaftNode
	client_chans map[uint64](chan *fs.Msg)
	commitChan *(chan raft.CommitInfo)
	fsys *fs.FileSystem
}

type ClientMessage struct {
	Uid uint64
	Message fs.Msg
}


func InitClientHandeler(rn *raft.RaftNode,fsys *fs.FileSystem) (*ClientHandeler) {
	ch := ClientHandeler{}
	//fmt.Println("RN:", rn,"FS", fsys)
	ch.rn = rn
	ch.client_chans = make(map[uint64](chan *fs.Msg))
	ch.commitChan = rn.GetCommitChannel()
	ch.fsys = fsys
	return &ch
}

func (ch *ClientHandeler) getChan(uid uint64) (*(chan *fs.Msg)) {
	ret := make(chan *fs.Msg,1)
	ch.client_chans[uid] = ret
	return &ret 
}

func (ch *ClientHandeler) processCommits() {
	for {
//fmt.Println("Jash")
		ci,_ := <- *ch.commitChan	//? Skipped Indexes , Redirects
		data := ClientMessage{}
		json.Unmarshal(ci.Data,&data)	//? error
		response :=  ch.fsys.ProcessMsg(&data.Message)
		ch.client_chans[data.Uid] <- response
	}
}


func check(obj interface{}) {
	if obj != nil {
		fmt.Println(obj)
		os.Exit(1)
	}
}

func reply(conn *net.TCPConn, msg *fs.Msg) bool {
	var err error
	write := func(data []byte) {
		if err != nil {
			return
		}
		_, err = conn.Write(data)
	}
	var resp string
	switch msg.Kind {
	case 'C': // read response
		resp = fmt.Sprintf("CONTENTS %d %d %d", msg.Version, msg.Numbytes, msg.Exptime)
	case 'O':
		resp = "OK "
		if msg.Version > 0 {
			resp += strconv.Itoa(msg.Version)
		}
	case 'F':
		resp = "ERR_FILE_NOT_FOUND"
	case 'V':
		resp = "ERR_VERSION " + strconv.Itoa(msg.Version)
	case 'M':
		resp = "ERR_CMD_ERR"
	case 'I':
		resp = "ERR_INTERNAL"
	default:
		fmt.Printf("Unknown response kind '%c'", msg.Kind)
		return false
	}
	resp += "\r\n"
	write([]byte(resp))
	if msg.Kind == 'C' {
		write(msg.Contents)
		write(crlf)
	}
	return err == nil
}

func serve(conn *net.TCPConn, uid uint64, receiveChan *(chan *fs.Msg),rn *raft.RaftNode) {
	reader := bufio.NewReader(conn)

	//replyChan := fs.GetChannel(uid)
	for {
		msg, msgerr, fatalerr := fs.GetMsg(reader)
		if fatalerr != nil || msgerr != nil {
			reply(conn, &fs.Msg{Kind: 'M'})
			conn.Close()
			break
		}

		if msgerr != nil {
			if (!reply(conn, &fs.Msg{Kind: 'M'})) {
				conn.Close()
				break
			}
		}

		//response := fsys.ProcessMsg(msg)
		data,_ := json.Marshal(ClientMessage{uid,*msg})	//? handle error reply(conn, &fs.Msg{Kind: 'I'})
		rn.Append(data)
		response,_ := <- *receiveChan //? error
		if !reply(conn, response) {
			conn.Close()
			break
		}
	}
}

//Old name func serverMain(socket string, rn *raft.RaftNode) {
func StartServer(socket string, conf raft.Config) {
	rn,err := raft.CreateRaftNode(conf)
	if err!= nil {
		//? Error
	}
	rn.Start()
	uid_counter := uint64(1)
	fsys := fs.GetFileSystem(1000)
	ch := InitClientHandeler(rn,fsys)
	//fmt.Println(ch)
	go ch.processCommits()
	

	tcpaddr, err := net.ResolveTCPAddr("tcp", socket)
	check(err)
	tcp_acceptor, err := net.ListenTCP("tcp", tcpaddr)
	check(err)

	for {
		tcp_conn, err := tcp_acceptor.AcceptTCP()
		check(err)
		go serve(tcp_conn,uid_counter,ch.getChan(uid_counter),rn)
		uid_counter++
	}
}

/*
func main() {
	var ids []int = []int{100, 200, 300, 400, 500}
	var nc []NetConfig = []NetConfig{{uint64(ids[0]), "localhost", uint16(5001)},
	{uint64(ids[1]), "localhost", uint16(5002)},
	{uint64(ids[2]), "localhost", uint16(5003)},
	{uint64(ids[3]), "localhost", uint16(5004)},
	{uint64(ids[4]), "localhost", uint16(5005)}}
	const EleTimeout uint64 = 5000
	const HBTimeout uint64 = 2000
	conf := raft.Config{nc, uint64(ids[idx]), DIRNAME + strconv.Itoa(int(ids[idx])), EleTimeout, HBTimeout}

	StartServer("localhost:8080", conf)
}
*/
