// ******************************************************************************************************
// * Took help from https://systembash.com/a-simple-go-tcp-server-and-tcp-client/  for client and server
// * edited by Jash Dave
// ******************************************************************************************************

package main

import "os"
import "net"
//import "fmt"
import "sync"
import "time"
import "bufio"
import "errors"
import "strings"
import "strconv"
import "io/ioutil"
import "encoding/json"

var data_dir string

type ConcurrencyManager struct {
	mx *sync.RWMutex
}

var cmMap map[string]ConcurrencyManager

type FileWrapper struct {
	Filename      string
	Version       uint64
	Numbytes      uint64
	Creation_time uint64
	Exptime       uint64
	Contents      []byte
}

func getCurrentTimeSeconds() uint64 {
	return uint64(time.Now().Unix())
}

func readFromFile(filename string) (FileWrapper, error) {
	if val,valid := cmMap[filename]; valid {
		val.mx.Lock()
	} else {
		cmMap[filename] = ConcurrencyManager{new(sync.RWMutex)}
		cmMap[filename].mx.Lock()
	}
	defer cmMap[filename].mx.Unlock()
	fw := new(FileWrapper)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return *fw, err
	}
	err = json.Unmarshal(data, fw)
	if err != nil {
		return *fw, err
	}
	return *fw, nil
}

func writeToFile(data_dir, filename string, version, numbytes, creation_time, exptime uint64, contents []byte) error {
	fn := data_dir+filename
	if val,valid := cmMap[fn]; valid {
		val.mx.Lock()
	} else {
		cmMap[fn] = ConcurrencyManager{new(sync.RWMutex)}
		cmMap[fn].mx.Lock()
	}
	defer cmMap[fn].mx.Unlock()

	fw := FileWrapper{filename, version, numbytes, creation_time, exptime, contents}
	data, err := json.Marshal(fw)
	if err != nil {
		return err
	}
	err = os.MkdirAll(data_dir, 0777)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(data_dir+filename, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func readAll(conn net.Conn) []byte {
	var message []byte
	tbuf := make([]byte, 256)
	creader := bufio.NewReader(conn)
	for {
		conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		n, err := creader.Read(tbuf)
		message = append(message, tbuf[:n]...)
		if err != nil {
			return message
		}
	}
	return message
}

func splitStringByLen(s string, l uint64) (string, string, error) {
	b_arr := []byte(s)
	if uint64(len(b_arr)) < l {
		return "", "", errors.New("Invalid length")
	}
	return string(b_arr[:l]), string(b_arr[l:]), nil
}

func doWrite(str_data string, conn net.Conn) (string, error) {
	str_arr := strings.SplitN(str_data, "\r\n", 2)
	if len(str_arr) != 2 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Write Error")
	}
	params := strings.Split(str_arr[0], " ")
	l := len(params)
	if l > 3 || l < 2 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Write Error")
	}

	exptime := uint64(0)
	filename := params[0]
	if len(filename) > 250 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Long filename")
	}

	numbytes, err := strconv.ParseUint(params[1], 10, 64)
	if err != nil {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Invalid content length")
	}

	if l == 3 {
		exptime, err = strconv.ParseUint(params[2], 10, 64)
		if err != nil {
			conn.Write([]byte("ERR_CMD_ERR\r\n"))
			//fmt.Println(err)
			return "", errors.New("Invalid exptime")
		}
	}

	contents, remaining, err := splitStringByLen(str_arr[1], numbytes)
	if err != nil {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Content length mismatch")
	}
	if strings.Index(remaining, "\r\n") != 0 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Content length mismatch")
	} else {
		_, remaining, _ = splitStringByLen(remaining, 2)
	}

	creation_time := getCurrentTimeSeconds()

	version := uint64(1)
	fw, err1 := readFromFile(data_dir + filename)
	if err1 == nil && (fw.Exptime == 0 || (fw.Exptime+fw.Creation_time) > getCurrentTimeSeconds()) {
		version = fw.Version + 1
	}

	err = writeToFile(data_dir, filename, version, numbytes, creation_time, exptime, []byte(contents))
	if err != nil {
		conn.Write([]byte("ERR_INTERNAL\r\n"))
		return "", err
	}

	conn.Write([]byte("OK " + strconv.FormatUint(version, 10) + "\r\n"))
	return remaining, nil
}


func doRead(str_data string, conn net.Conn) (string, error) {

	str_arr := strings.SplitN(str_data, "\r\n", 2)
	remaining := ""
	if len(str_arr) == 2 {
		remaining = str_arr[1]
	}

	//---Not necessary to check----
	if strings.Index(str_arr[0], " ") != -1 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Spaces in filename")
	}
	//-----------------------------

	filename := str_arr[0]
	if len(filename) > 250 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Filename excedes length")
	}

	fw, err := readFromFile(data_dir + filename)
	if err != nil {
		conn.Write([]byte("ERR_FILE_NOT_FOUND\r\n"))
		return "", err
	} else {
		if fw.Exptime != 0 && (fw.Exptime+fw.Creation_time) < getCurrentTimeSeconds() {
			conn.Write([]byte("ERR_FILE_NOT_FOUND\r\n"))
			return "", errors.New("File expired")
		}
	}

	data := "CONTENTS "
	data = data + strconv.FormatUint(fw.Version, 10) + " "
	data = data + strconv.FormatUint(fw.Numbytes, 10) + " "
	data = data + strconv.FormatUint(fw.Exptime, 10) + " \r\n"
	data = data + string(fw.Contents) + "\r\n"
	//fmt.Println("S Send:",data)
	conn.Write([]byte(data))
	return remaining, nil
}

func doCAS(str_data string, conn net.Conn) (string, error) {

	str_arr := strings.SplitN(str_data, "\r\n", 2)
	if len(str_arr) != 2 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Command Error")
	}
	params := strings.Split(str_arr[0], " ")

	l := len(params)
	if l > 4 || l < 3 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Invalid no of params")
	}

	exptime := uint64(0)
	filename := params[0]
	if len(filename) > 250 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Filename excedes length")
	}

	version, err := strconv.ParseUint(params[1], 10, 64)
	if err != nil {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Invalid version")
	}

	numbytes, err := strconv.ParseUint(params[2], 10, 64)
	if err != nil {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Invalid numbytes")
	}

	if l == 4 {
		exptime, err = strconv.ParseUint(params[3], 10, 64)
		if err != nil {
			conn.Write([]byte("ERR_CMD_ERR\r\n"))
			return "", errors.New("Invalid exptime")
		}
	}

	contents, remaining, err := splitStringByLen(str_arr[1], numbytes)
	if err != nil {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Content length mismatch")
	}
	if strings.Index(remaining, "\r\n") != 0 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Content length mismatch")
	} else {
		_, remaining, _ = splitStringByLen(remaining, 2)
	}

	fw, err1 := readFromFile(data_dir + filename)
	if err1 != nil {
		conn.Write([]byte("ERR_FILE_NOT_FOUND\r\n"))
		return "", err
	} else {
		if fw.Exptime > 0 && (fw.Exptime+fw.Creation_time) < getCurrentTimeSeconds() {
			os.Remove(data_dir + filename)
			conn.Write([]byte("ERR_FILE_NOT_FOUND\r\n"))
			return "", errors.New("File expired + deleted")
		}
	}
	if fw.Version != version {
		conn.Write([]byte("ERR_VERSION\r\n"))
		return "", errors.New("Version mismatch")
	}

	creation_time := getCurrentTimeSeconds()

	err = writeToFile(data_dir, filename, version, numbytes, creation_time, exptime, []byte(contents))
	if err != nil {
		conn.Write([]byte("ERR_INTERNAL\r\n"))
		return "", err
	}

	conn.Write([]byte("OK " + strconv.FormatUint(version, 10) + "\r\n"))
	return remaining, nil
}

func doDelete(str_data string, conn net.Conn) (string, error) {
	str_arr := strings.SplitN(str_data, "\r\n", 2)
	remaining := ""
	if len(str_arr) == 2 {
		remaining = str_arr[1]
	}

	//---Not necessary to check----
	if strings.Index(str_arr[0], " ") != -1 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Spaces in filename")
	}
	//-----------------------------

	filename := str_arr[0]
	if len(filename) > 250 {
		conn.Write([]byte("ERR_CMD_ERR\r\n"))
		return "", errors.New("Filename excedes length")
	}

	if _, err := os.Stat(data_dir + filename); os.IsNotExist(err) {
		conn.Write([]byte("ERR_FILE_NOT_FOUND\r\n"))
		return "", err
	}
	if os.Remove(data_dir+filename) != nil {
		conn.Write([]byte("ERR_INTERNAL\r\n"))
		return "", errors.New("Error on delete")
	}
	conn.Write([]byte("OK\r\n"))
	return remaining, nil
}

func handleClient(conn net.Conn) {
	str_data := ""
	i := 1
	for {
		if len(str_data) == 0 {
			l := 0
			for l == 0 {
				byte_data := readAll(conn)
				str_data = str_data + string(byte_data)
				l = len(byte_data)
				if l == 0 {
					time.Sleep(50 * time.Millisecond)
				}
			}
		}

		i++
		str_arr := strings.SplitN(str_data, " ", 2)
		if len(str_arr) != 2 {
			//conn.Write([]byte("ERR_INTERNAL\r\n"))  //?????
			continue
		}
		command := str_arr[0]
		str_data = str_arr[1]

		switch command {
		case "write":
			//fmt.Println("Do Write")
			str_data, _ = doWrite(str_data, conn)
		case "read":
			//fmt.Println("Do Read")
			str_data, _ = doRead(str_data, conn)
		case "cas":
			//fmt.Println("Do CAS")
			str_data, _ = doCAS(str_data, conn)
		case "delete":
			//fmt.Println("Do Delete")
			str_data, _ = doDelete(str_data, conn)
		default:
			str_data = ""
			conn.Write([]byte("ERR_INTERNAL\r\n"))
		}
		//fmt.Println("Remaining : ",str_data)
	}

}

func serverMain() {
	cmMap = make(map[string]ConcurrencyManager)
	data_dir = os.Getenv("GOPATH") + "/cs733_data_files/assign1/"
	//fmt.Println("Launching server...")
	ln, _ := net.Listen("tcp", ":8080")
	for {
		conn, err := ln.Accept()
		if err != nil {
			//fmt.Printf("%v",err)
			continue
		}
		go handleClient(conn)
	}
}

func main() {
	serverMain()
}
