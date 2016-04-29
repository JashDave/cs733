#Introduction
This is a 
This is implementation of Raft consensus algorithm by Diego Ongaro and John Ousterhout(Stanford University), by Jash Dave for course CS733 at IITB (Spring 2015-16).

For more details of Raft consensus algorithm refer https://ramcloud.stanford.edu/raft.pdf


#Installation
go get github.com/JashDave/cs733/assignment4


#Dependences
	ZeroMQ form http://zeromq.org/
	"github.com/cs733-iitb/log"
	"github.com/cs733-iitb/cluster"

Thanks to ZeroMQ authors and Sriram Srinivasan for providing log and cluster libraries.

#Usage
go run server.go configuration_file


#Yet to be made

#Few Notes
rpc_test.go test cases takes about 350s to run on my computer

I have changed nclients value form 500 to 50 in `TestRPC_ConcurrentWrites()` since it was taking too much time.

Also I have changed `TestRPC_ConcurrentCas(t *testing.T)`
```
	nclients := 3 //from #100 to 3
	niters := 2   //from #10 to 2
	if !(m.Kind == 'C' && strings.HasSuffix(string(m.Contents), " 1")) {
```

#Thank Note
File system and Server code are taken form "github.com/cs733-iitb/cs733/assignment1" and edited by me.
Thank you Sriram Sir.
