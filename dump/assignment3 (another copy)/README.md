#Introduction
This is implementation of Raft consensus algorithm by Diego Ongaro and John Ousterhout(Stanford University), by Jash Dave for course CS733 at IITB (Spring 2015-16).
For more details of Raft consensus algorithm refer https://ramcloud.stanford.edu/raft.pdf

#Usage
```
import . "github.com/JashDave/cs733/assignment3/"

//Network Configuration of RaftNodes as ID, HostAddress, PortNumber
var nc []NetConfig = []NetConfig{100, "localhost", uint16(5001)}, 
						{200, "localhost", uint16(5002)}, 
						{300, "localhost", uint16(5003)}, 
						{400, "localhost", uint16(5004)}, 
						{500, "localhost", uint16(5005)}}

//RaftNode configuration
conf := Config{NetConfig, ID, "LogDirName", ElectionTimeout, HeartbeatTimeout}
eg:
conf := Config{nc, 100, "LogDirName", 5000, 200}

//Creates raft node for given configuration
rn1,err := raft.CreateRaftNode(conf)	

//Create other nodes also
For ID = {200, 300, 400, 500}
conf := Config{nc, ID, "LogDirName"+ID, 5000, 200}
rnID,err := raft.CreateRaftNode(conf)
	
//Start the RaftNode
rn.Start()	//Must be called only once after creation

//Get Leader Id
leader_id := rn.LeaderId()

//Search leader instance by its id
if(rn.Id() == leader_id) { //for majority
	leader_rn = rn
}

//Append some data
leader_rn.Append([]byte("Your Data as byte array"))

//Get Commit Response
response := <- *rn.GetCommitChannel()

//Get Commit Index
commit_index := rn.CommittedIndex()

//To shutdown the node
rn.Shutdown()

#Testing
Just do 
go test github.com/cs733/assignment3
It takes around 100 seconds to run. 


#Dependences

	"github.com/cs733-iitb/log"
	"github.com/cs733-iitb/cluster"
	"github.com/JashDave/cs733/assignment3/assignment2"

Thanks to Sriram Srinivasan for providing log and cluster libraries.


#NOTE
This implementation is still under development and the users must use this on their own risk.

