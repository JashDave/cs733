package main


import (

	"bufio"
	"bytes"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"

	"testing"
	"time"
	"os"
	"fmt"
	//raft "github.com/JashDave/cs733/assignment4/assignment3"
)

var Servers []*os.Process

func TestRPCMain(t *testing.T) {
	Servers = make([]*os.Process,5)
	os.RemoveAll("TestLog100")
	os.RemoveAll("TestLog200")
	os.RemoveAll("TestLog300")
	os.RemoveAll("TestLog400")
	os.RemoveAll("TestLog500")
	for idx:=0;idx<5;idx++ {
		time.Sleep(2000 *time.Millisecond)
		cmd := "/usr/local/go/bin/go"
		args := []string{"","run","server.go",fmt.Sprintf("Config/ServerConf%d.json",idx)}
		pattr := new(os.ProcAttr)
		pattr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
		p, err := os.StartProcess(cmd, args, pattr)
		if err != nil {
			fmt.Printf("Error running %s:\n%s\n", cmd, err.Error())
		} else {
			Servers[idx]=p
			//fmt.Printf("PID for %s is %d\n", cmd, p.Pid)
		}

	}
fmt.Println("Initiated 1")
	time.Sleep(time.Duration(5000) * 10 *time.Millisecond)
fmt.Println("Initiated 2")
}

func expect(t *testing.T, response *Msg, expected *Msg, errstr string, err error) {
	if err != nil {
		t.Fatal("Unexpected error: " + err.Error())
	}
	ok := true
	if response.Kind != expected.Kind {
		ok = false
		errstr += fmt.Sprintf(" Got kind='%c', expected '%c'", response.Kind, expected.Kind)
	}
	if expected.Version > 0 && expected.Version != response.Version {
		ok = false
		errstr += " Version mismatch"
	}
	if response.Kind == 'C' {
		if expected.Contents != nil &&
			bytes.Compare(response.Contents, expected.Contents) != 0 {
			ok = false
		}
	}
	if !ok {
		t.Fatal("Expected " + errstr)
	}
}



func TestRPC_BasicSequential(t *testing.T) {
	cl := mkClient(t)
	defer cl.close()

fmt.Println("DP1")
	// Read non-existent file cs733net
	m, err := cl.read("cs733net")
	expect(t, m, &Msg{Kind: 'F'}, "file not found", err)

	// Read non-existent file cs733net
	m, err = cl.delete("cs733net")
	expect(t, m, &Msg{Kind: 'F'}, "file not found", err)

	// Write file cs733net
	data := "Cloud fun"
	m, err = cl.write("cs733net", data, 0)
	expect(t, m, &Msg{Kind: 'O'}, "write success", err)

	// Expect to read it back
	m, err = cl.read("cs733net")
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(data)}, "read my write", err)

	// CAS in new value
	version1 := m.Version
	data2 := "Cloud fun 2"
	// Cas new value
	m, err = cl.cas("cs733net", version1, data2, 0)
	expect(t, m, &Msg{Kind: 'O'}, "cas success", err)

//fmt.Println("DP2")
	// Expect to read it back
	m, err = cl.read("cs733net")
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(data2)}, "read my cas", err)

	// Expect Cas to fail with old version
	m, err = cl.cas("cs733net", version1, data, 0)
	expect(t, m, &Msg{Kind: 'V'}, "cas version mismatch", err)

	// Expect a failed cas to not have succeeded. Read should return data2.
	m, err = cl.read("cs733net")
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(data2)}, "failed cas to not have succeeded", err)

	// delete
	m, err = cl.delete("cs733net")
	expect(t, m, &Msg{Kind: 'O'}, "delete success", err)

	// Expect to not find the file
	m, err = cl.read("cs733net")
	expect(t, m, &Msg{Kind: 'F'}, "file not found", err)

//fmt.Println("DP3")
}

func TestRPC_Binary(t *testing.T) {
	cl := mkClient(t)
	defer cl.close()


//fmt.Println("DP4")
	// Write binary contents
	data := "\x00\x01\r\n\x03" // some non-ascii, some crlf chars
	m, err := cl.write("binfile", data, 0)
	expect(t, m, &Msg{Kind: 'O'}, "write success", err)

	// Expect to read it back
	m, err = cl.read("binfile")
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(data)}, "read my write", err)

//fmt.Println("DP5")
}

func TestRPC_Chunks(t *testing.T) {
	// Should be able to accept a few bytes at a time
	cl := mkClient(t)
	defer cl.close()
	var err error
	snd := func(chunk string) {
		if err == nil {
			err = cl.send(chunk)
		}
	}


//fmt.Println("DP6")
	// Send the command "write teststream 10\r\nabcdefghij\r\n" in multiple chunks
	// Nagle's algorithm is disabled on a write, so the server should get these in separate TCP packets.
	snd("wr")
	time.Sleep(10 * time.Millisecond)
	snd("ite test")
	time.Sleep(10 * time.Millisecond)
	snd("stream 1")
	time.Sleep(10 * time.Millisecond)
	snd("0\r\nabcdefghij\r")
	time.Sleep(10 * time.Millisecond)
	snd("\n")
	var m *Msg
	m, err = cl.rcv()
	expect(t, m, &Msg{Kind: 'O'}, "writing in chunks should work", err)

//fmt.Println("DP7")
}

func TestRPC_Batch(t *testing.T) {
	// Send multiple commands in one batch, expect multiple responses
	cl := mkClient(t)
	defer cl.close()
	cmds := "write batch1 3\r\nabc\r\n" +
		"write batch2 4\r\ndefg\r\n" +
		"read batch1\r\n"

//fmt.Println("DP8")
	cl.send(cmds)
	m, err := cl.rcv()
	expect(t, m, &Msg{Kind: 'O'}, "write batch1 success", err)
	m, err = cl.rcv()
	expect(t, m, &Msg{Kind: 'O'}, "write batch2 success", err)
	m, err = cl.rcv()
	expect(t, m, &Msg{Kind: 'C', Contents: []byte("abc")}, "read batch1", err)

//fmt.Println("DP9")
}

func TestRPC_BasicTimer(t *testing.T) {
	cl := mkClient(t)
	defer cl.close()

	// Write file cs733, with expiry time of 2 seconds
	str := "Cloud fun"
	m, err := cl.write("cs733", str, 2)
	expect(t, m, &Msg{Kind: 'O'}, "write success", err)

	// Expect to read it back immediately.
	m, err = cl.read("cs733")
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(str)}, "read my cas", err)

//fmt.Println("DP10")
	time.Sleep(3 * time.Second)

	// Expect to not find the file after expiry
	m, err = cl.read("cs733")
	expect(t, m, &Msg{Kind: 'F'}, "file not found", err)

	// Recreate the file with expiry time of 1 second
	m, err = cl.write("cs733", str, 1)
	expect(t, m, &Msg{Kind: 'O'}, "file recreated", err)

	// Overwrite the file with expiry time of 4. This should be the new time.
	m, err = cl.write("cs733", str, 3)
	expect(t, m, &Msg{Kind: 'O'}, "file overwriten with exptime=4", err)

	// The last expiry time was 3 seconds. We should expect the file to still be around 2 seconds later
	time.Sleep(2 * time.Second)

//fmt.Println("DP11")
	// Expect the file to not have expired.
	m, err = cl.read("cs733")
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(str)}, "file to not expire until 4 sec", err)

	time.Sleep(3 * time.Second)
	// 5 seconds since the last write. Expect the file to have expired
	m, err = cl.read("cs733")
	expect(t, m, &Msg{Kind: 'F'}, "file not found after 4 sec", err)

	// Create the file with an expiry time of 1 sec. We're going to delete it
	// then immediately create it. The new file better not get deleted. 
	m, err = cl.write("cs733", str, 1)
	expect(t, m, &Msg{Kind: 'O'}, "file created for delete", err)

//fmt.Println("DP12")
	m, err = cl.delete("cs733")
	expect(t, m, &Msg{Kind: 'O'}, "deleted ok", err)

	m, err = cl.write("cs733", str, 0) // No expiry
	expect(t, m, &Msg{Kind: 'O'}, "file recreated", err)

	time.Sleep(1100 * time.Millisecond) // A little more than 1 sec
	m, err = cl.read("cs733")
	expect(t, m, &Msg{Kind: 'C'}, "file should not be deleted", err)

//fmt.Println("DP12.1")
}


// nclients write to the same file. At the end the file should be
// any one clients' last write

func TestRPC_ConcurrentWrites(t *testing.T) {
	nclients := 10 //? #500 was here
	niters := 10
	clients := make([]*Client, nclients)

//fmt.Println("DP13.1")
	for i := 0; i < nclients; i++ {
		cl := mkClient(t)
		if cl == nil {
			t.Fatalf("Unable to create client #%d", i)
		}
		defer cl.close()
		clients[i] = cl
	}

//fmt.Println("DP13")

	errCh := make(chan error, nclients)
	var sem sync.WaitGroup // Used as a semaphore to coordinate goroutines to begin concurrently
	sem.Add(1)
	ch := make(chan *Msg, nclients*niters) // channel for all replies
	for i := 0; i < nclients; i++ {
		go func(i int, cl *Client) {
			sem.Wait()
			for j := 0; j < niters; j++ {
				str := fmt.Sprintf("cl %d %d", i, j)
				m, err := cl.write("concWrite", str, 0)
				if err != nil {
					errCh <- err
					break
				} else {
					ch <- m
				}
			}
		}(i, clients[i])
	}
	time.Sleep(100 * time.Millisecond) // give goroutines a chance
	sem.Done()                         // Go!

//fmt.Println("DP14")
	// There should be no errors
	for i := 0; i < nclients*niters; i++ {
		select {
		case m := <-ch:
			if m.Kind != 'O' {
				t.Fatalf("Concurrent write failed with kind=%c", m.Kind)
			}
		case err := <- errCh:
			t.Fatal(err)
		}
	}
	m, _ := clients[0].read("concWrite")
	// Ensure the contents are of the form "cl <i> 9"
	// The last write of any client ends with " 9"
	if !(m.Kind == 'C' && strings.HasSuffix(string(m.Contents), " 9")) {
		t.Fatalf("Expected to be able to read after 1000 writes. Got msg = %v", m)
	}

//fmt.Println("DP14.1")
}


// nclients cas to the same file. At the end the file should be any one clients' last write.
// The only difference between this test and the ConcurrentWrite test above is that each
// client loops around until each CAS succeeds. The number of concurrent clients has been
// reduced to keep the testing time within limits.
func TestRPC_ConcurrentCas(t *testing.T) {
	nclients := 3 //? #100
	niters := 2   //? #10

//fmt.Println("DP15.1")
	clients := make([]*Client, nclients)
	for i := 0; i < nclients; i++ {
		cl := mkClient(t)
		if cl == nil {
			t.Fatalf("Unable to create client #%d", i)
		}
		defer cl.close()
		clients[i] = cl
	}

//fmt.Println("DP15")
	var sem sync.WaitGroup // Used as a semaphore to coordinate goroutines to *begin* concurrently
	sem.Add(1)

	m, _ := clients[0].write("concCas", "first", 0)
	ver := m.Version
	if m.Kind != 'O' || ver == 0 {
		t.Fatalf("Expected write to succeed and return version")
	}

	var wg sync.WaitGroup
	wg.Add(nclients)

	errorCh := make(chan error, nclients)
	
	for i := 0; i < nclients; i++ {
		go func(i int, ver int, cl *Client) {
//fmt.Println("15.2 i",i)
			sem.Wait()
			defer wg.Done()
			for j := 0; j < niters; j++ {

//fmt.Println("15.3 ij",i,j)
				str := fmt.Sprintf("cl %d %d", i, j)
				for {
					m, err := cl.cas("concCas", ver, str, 0)
					if err != nil {
						errorCh <- err
						return
					} else if m.Kind == 'O' {
						break
					} else if m.Kind != 'V' {
						errorCh <- errors.New(fmt.Sprintf("Expected 'V' msg, got %c", m.Kind))
						return
					}
					ver = m.Version // retry with latest version
//fmt.Println("15.4 ij",i,j)
				}
			}
		}(i, ver, clients[i])
	}

	time.Sleep(100 * time.Millisecond) // give goroutines a chance
	sem.Done()                         // Start goroutines
	wg.Wait()                          // Wait for them to finish
	select {
	case e := <- errorCh:
		t.Fatalf("Error received while doing cas: %v", e)
	default: // no errors
	}
	m, _ = clients[0].read("concCas")
//	if !(m.Kind == 'C' && strings.HasSuffix(string(m.Contents), " 9")) {	//Original
	if !(m.Kind == 'C' && strings.HasSuffix(string(m.Contents), " 1")) {
		t.Fatalf("Expected to be able to read after 1000 writes. Got msg.Kind = %d, msg.Contents=%s", m.Kind, m.Contents)
	}
}


func TestRPC_RaftStability(t *testing.T) {

	defer Servers[1].Kill()
	defer mykill(8081)
	defer Servers[2].Kill()
	defer mykill(8082)
	defer Servers[3].Kill()
	defer mykill(8083)
	defer Servers[4].Kill()
	defer mykill(8084)


	cl := mkClient(t)
fmt.Println("DP50")
	data := "My Data."
	m, err := cl.write("StabilityTestFile", data, 0)
	expect(t, m, &Msg{Kind: 'O'}, "write success", err)

	// Expect to read it back
	m, err = cl.read("StabilityTestFile")
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(data)}, "read my write", err)
	cl.close()

fmt.Println("DP51  ",Servers[0].Pid)
	err2:=Servers[0].Kill()
	err3:=Servers[0].Release()
mykill(8080)


//Wait for new leader to get elected
fmt.Println("DP52  ",Servers[0].Pid,err2,err3)
	time.Sleep(time.Duration(5000) * 5 *time.Millisecond)


fmt.Println("DP53")
	cl2 := mkClient2(t,"localhost:8083")
	defer cl2.close()

fmt.Println("DP54")
	m, err = cl2.read("StabilityTestFile")

fmt.Println("DP55")
	if m.Kind=='R' {
fmt.Println("Redirected to",m.Filename)
		cl3 := mkClient2(t,m.Filename)

fmt.Println("DP56")
		m, err = cl3.read("StabilityTestFile")

fmt.Println("DP57")
		expect(t, m, &Msg{Kind: 'C', Contents: []byte(data)}, "read my write"+m.Filename, err)
		defer cl3.close()
	} else {
		expect(t, m, &Msg{Kind: 'C', Contents: []byte(data)}, "read my write", err)
	}

fmt.Println("DP58")


}

func mykill(port int) {
	cmd := "/bin/fuser"
	args := []string{"","-k",fmt.Sprintf("%d/tcp",port)}
	pattr := new(os.ProcAttr)
	pattr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
	_, err := os.StartProcess(cmd, args, pattr);
	if err != nil {
	    fmt.Printf("Error in running %s : \n%s\n", cmd, err.Error())
	} else {
	    //fmt.Printf("PID for %s id %d\n", cmd, p.Pid)
	}
}
//----------------------------------------------------------------------
// Utility functions

type Msg struct {
	// Kind = the first character of the command. For errors, it
	// is the first letter after "ERR_", ('V' for ERR_VERSION, for
	// example), except for "ERR_CMD_ERR", for which the kind is 'M'
	Kind     byte
	Filename string
	Contents []byte
	Numbytes int
	Exptime  int // expiry time in seconds
	Version  int
}

func (cl *Client) read(filename string) (*Msg, error) {
	cmd := "read " + filename + "\r\n"
	return cl.sendRcv(cmd)
}

func (cl *Client) write(filename string, contents string, exptime int) (*Msg, error) {
	var cmd string
	if exptime == 0 {
		cmd = fmt.Sprintf("write %s %d\r\n", filename, len(contents))
	} else {
		cmd = fmt.Sprintf("write %s %d %d\r\n", filename, len(contents), exptime)
	}
	cmd += contents + "\r\n"
	return cl.sendRcv(cmd)
}

func (cl *Client) cas(filename string, version int, contents string, exptime int) (*Msg, error) {
	var cmd string
	if exptime == 0 {
		cmd = fmt.Sprintf("cas %s %d %d\r\n", filename, version, len(contents))
	} else {
		cmd = fmt.Sprintf("cas %s %d %d %d\r\n", filename, version, len(contents), exptime)
	}
	cmd += contents + "\r\n"
	return cl.sendRcv(cmd)
}

func (cl *Client) delete(filename string) (*Msg, error) {
	cmd := "delete " + filename + "\r\n"
	return cl.sendRcv(cmd)
}

var errNoConn = errors.New("Connection is closed")

type Client struct {
	conn   *net.TCPConn
	reader *bufio.Reader // a bufio Reader wrapper over conn
}

func mkClient(t *testing.T) *Client {
	var client *Client
	raddr, err := net.ResolveTCPAddr("tcp", "localhost:8080")
	if err == nil {
		conn, err := net.DialTCP("tcp", nil, raddr)
		if err == nil {
			client = &Client{conn: conn, reader: bufio.NewReader(conn)}
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func mkClient2(t *testing.T,addr string) *Client {
	var client *Client
	raddr, err := net.ResolveTCPAddr("tcp", addr)
	if err == nil {
		conn, err := net.DialTCP("tcp", nil, raddr)
		if err == nil {
			client = &Client{conn: conn, reader: bufio.NewReader(conn)}
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func (cl *Client) send(str string) error {
	if cl.conn == nil {
		return errNoConn
	}
	_, err := cl.conn.Write([]byte(str))
	if err != nil {
		err = fmt.Errorf("Write error in SendRaw: %v", err)
		cl.conn.Close()
		cl.conn = nil
	}
	return err
}

func (cl *Client) sendRcv(str string) (msg *Msg, err error) {
	if cl.conn == nil {
		return nil, errNoConn
	}
	err = cl.send(str)
	if err == nil {
		msg, err = cl.rcv()
	}
	return msg, err
}

func (cl *Client) close() {
	if cl != nil && cl.conn != nil {
		cl.conn.Close()
		cl.conn = nil
	}
}

func (cl *Client) rcv() (msg *Msg, err error) {
	// we will assume no errors in server side formatting
	line, err := cl.reader.ReadString('\n')
	if err == nil {
		msg, err = parseFirst(line)
		if err != nil {
			return nil, err
		}
		if msg.Kind == 'C' {
			contents := make([]byte, msg.Numbytes)
			var c byte
			for i := 0; i < msg.Numbytes; i++ {
				if c, err = cl.reader.ReadByte(); err != nil {
					break
				}
				contents[i] = c
			}
			if err == nil {
				msg.Contents = contents
				cl.reader.ReadByte() // \r
				cl.reader.ReadByte() // \n
			}
		}
	}
	if err != nil {
		cl.close()
	}
	return msg, err
}

func parseFirst(line string) (msg *Msg, err error) {
	fields := strings.Fields(line)
	msg = &Msg{}

	// Utility function fieldNum to int
	toInt := func(fieldNum int) int {
		var i int
		if err == nil {
			if fieldNum >=  len(fields) {
				err = errors.New(fmt.Sprintf("Not enough fields. Expected field #%d in %s\n", fieldNum, line))
				return 0
			}
			i, err = strconv.Atoi(fields[fieldNum])
		}
		return i
	}

	if len(fields) == 0 {
		return nil, errors.New("Empty line. The previous command is likely at fault")
	}
	switch fields[0] {
	case "OK": // OK [version]
		msg.Kind = 'O'
		if len(fields) > 1 {
			msg.Version = toInt(1)
		}
	case "CONTENTS": // CONTENTS <version> <numbytes> <exptime> \r\n
		msg.Kind = 'C'
		msg.Version = toInt(1)
		msg.Numbytes = toInt(2)
		msg.Exptime = toInt(3)
	case "ERR_VERSION":
		msg.Kind = 'V'
		msg.Version = toInt(1)
	case "ERR_FILE_NOT_FOUND":
		msg.Kind = 'F'
	case "ERR_CMD_ERR":
		msg.Kind = 'M'
	case "ERR_INTERNAL":
		msg.Kind = 'I'
	case "ERR_REDIRECT":
		msg.Kind = 'R'
		msg.Filename = fields[1]
	default:
		err = errors.New("Unknown response " + fields[0])
	}
	if err != nil {
		return nil, err
	} else {
		return msg, nil
	}
}
