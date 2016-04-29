##Introduction
This is an implementation of replicated network file system. Currently the given filesystem provides the basic facility of read, write, delete and compare-and-swap along with expiry time support.

It uses my implementation of Raft consensus algorithm for replication. Raft is desigined by Diego Ongaro and John Ousterhout(Stanford University). For more details of Raft consensus algorithm refer https://ramcloud.stanford.edu/raft.pdf


##Installation
```
go get github.com/JashDave/cs733/assignment4
```

For dependencies do
```
go get github.com/cs733-iitb/log
go get github.com/cs733-iitb/cluster
```

For other dependencies
```
go get github.com/syndtr/goleveldb
go get github.com/hashicorp/golang-lru
Download and install ZeroMQ form http://zeromq.org/
```


##Dependencies
1. ZeroMQ form http://zeromq.org/
2. Log from github.com/cs733-iitb/log
3. Cluster from github.com/cs733-iitb/cluster
4. It also uses an edited version of file system form github.com/cs733-iitb/cs733/assignment1/ as present in "fs" and "root" folder

##Testing
```
go test -v
```

##Usage
Start the required number of servers (atleast 3) with the following command.
```
go run server.go configuration_file
```
Example configuration files are present in "Config" folder.


##Example Usage

```
> go run server.go Config/ServerConf0.json &
> go run server.go Config/ServerConf1.json &
> go run server.go Config/ServerConf2.json &
> go run server.go Config/ServerConf3.json &
> go run server.go Config/ServerConf4.json &

> telnet localhost 8080
  Connected to localhost.
  Escape character is '^]'
  read myfile
  ERR_FILE_NOT_FOUND
  write myfile 6
  qwerty
  OK 1
  read foo
  CONTENTS 1 6 0
  qwerty
  cas myfile 1 4 0
  asdf
  OK 2
  read myfile
  CONTENTS 2 4 0
  asdf
```

##Command Specification

The format for each of the four commands is shown below,


| Command                                                                     | Success Response                                                                    | Error Response            |
|-----------------------------------------------------------------------------|-------------------------------------------------------------------------------------|---------------------------|
| read _filename_ \r\n                                                        | CONTENTS _version_ _numbytes_ _exptime remaining_\r\n</br>_content bytes_\r\n </br> | ERR_FILE_NOT_FOUND        |
| write _filename_ _numbytes_ [_exptime_]\r\n</br>_content bytes_\r\n         | OK _version_\r\n                                                                    |                           |
| cas _filename_ _version_ _numbytes_ [_exptime_]\r\n</br>_content bytes_\r\n | OK _version_\r\n                                                                    | ERR\_VERSION _newversion_ |
| delete _filename_ \r\n                                                      | OK\r\n                                                                              | ERR_FILE_NOT_FOUND        |

In addition the to the semantic error responses in the table above, all commands can get three additional errors. `ERR_CMD_ERR` is returned on a malformed command, `ERR_INTERNAL` on, well, internal errors and `ERR_REDIRECT socket_address`. On getting `ERR_REDIRECT` close your current telnet connection and connect to given socket address.

For `write` and `cas` and in the response to the `read` command, the content bytes is on a separate line. The length is given by _numbytes_ in the first line.

Files can have an optional expiry time, _exptime_, expressed in seconds. A subsequent `cas` or `write` cancels an earlier expiry time, and imposes the new time. By default, _exptime_ is 0, which represents no expiry. 

(This text section is from github.com/cs733-iitb/cs733/assignment1/README.md and edited by me)

##Thank Note
Thanks to ZeroMQ authors and Sriram Srinivasan for providing log, cluster and filesystem libraries.
