/*
 * Written by: Sagar Sontakke
 * Description: This is a clustering library that provide some abstractions for the setting up clusters.
 * It provides functions related to parsing input config file, setting up a server, input/output message
 * channels for the server, send/receive message functions, structure for server/mesage objects
 */

package cluster

import (
	"bufio"
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
)

var Peers []int
var PeerAddress = make(map[int]string)
var peerIdIndex = make(map[int]int)

const (
	BROADCAST = -1
)

/*
 * The structure to encapsulate the message, contains:
 * 1. Node is of receipient 2. Unique ID for the message 3. Actual message
 */

type Envelope struct {
	Pid   int
	MsgId int64
	Msg   string
}

/*
 * Structure for server object. Contains the server specific info like ID of that server,
 * server ip/port address, IDs and address of peer servers, message input/output channels
 * message ID counts, server's own socket and sockets for all its peers.
 */

type servernode struct {
	ServerId         int
	ServerAddress    string
	PeersId          []int
	PeerServers      map[int]string
	Serveroutchannel chan *Envelope
	Serverinchannel  chan *Envelope
	MessageId        int64
	Serversocket     *zmq.Socket
	Peersockets      []*zmq.Socket
}

/*
 * The server object impliments the Server interface
 */

type Server interface {
	GetPid() int
	GetPeers() []int
	Outbox() chan *Envelope
	Inbox() chan *Envelope
}

/* 	 Get PID of the server
 */

func (s servernode) GetPid() int {
	return s.ServerId
}

/*	Get IDs of the peer nodes
 */

func (s servernode) GetPeers() []int {
	return s.PeersId
}

/*	Set up a new cluster and retuern an array of object for the nodes
 */

func NewCluster(configFile string) []servernode {

	parsedConfig := false
	if !parsedConfig {
		if ParseConfig(configFile) != 0 {
			fmt.Println(": Error while parsing the config file")
			os.Exit(1)
		} else {
			parsedConfig = true
		}
	}

	var s servernode
	var serv []servernode
	var err error

	for i := 0; i < len(Peers); i++ {
		nodeid := Peers[i]
		s.ServerId = nodeid
		s.ServerAddress = PeerAddress[nodeid]
		s.PeersId = Peers
		s.PeerServers = PeerAddress
		s.Serveroutchannel = make(chan *Envelope)
		s.Serverinchannel = make(chan *Envelope)
		s.MessageId = int64(nodeid * 1000)
		s.Serversocket, err = zmq.NewSocket(zmq.PULL)
		check_err(err)
		s.Serversocket.Bind(s.ServerAddress)
		serv = append(serv, s)
	}

	//time.Sleep(1 * time.Second)

	for i := 0; i < len(serv); i++ {
		for j := 0; j < len(serv); j++ {
			sock, err := zmq.NewSocket(zmq.PUSH)
			check_err(err)
			serv[i].Peersockets = append(serv[i].Peersockets, sock)
			serv[i].Peersockets[j].Connect(serv[i].PeerServers[serv[j].ServerId])
		}
	}

	return serv
}

/*	Queue the Inbox channel for a server
 */

func QueueInbox(s servernode, e Envelope, in chan *Envelope) {
	in <- &e
}

/*	Return the Inbox channel for a server
 */

func (s servernode) Inbox() chan *Envelope {
	return s.Serverinchannel
}

/*	Return the Outbox channel for a server
 */

func (s servernode) Outbox() chan *Envelope {
	return s.Serveroutchannel
}

/*	Send messages that are queued in the outbox channel to the correspnding servers
 */

func SendMessages(s servernode) {
	for {
		select {
		case msg, valid := <-s.Outbox():
			if !valid {
				return
			} else {
				targetID := msg.Pid
				if targetID == BROADCAST {
					for i := 0; i < len(Peers); i++ {
						msg.Pid = Peers[i]
						msg.MsgId = s.MessageId
						atomic.AddInt64(&s.MessageId, 1)
						_, err := s.Peersockets[peerIdIndex[msg.Pid]].SendMessage(msg)
						check_err(err)
						//fmt.Println(n, "and", len(msg.Msg))
						//fmt.Println("SENDING: (Src,Dst) --> (",s.ServerId,",",msg.Pid,") (Message:",*msg,")")
					}
				} else {
					targetID = peerIdIndex[msg.Pid]
					msg.MsgId = s.MessageId
					atomic.AddInt64(&s.MessageId, 1)
					_, err := s.Peersockets[targetID].SendMessage(msg)
					//fmt.Println(n, "and", msg)
					check_err(err)
					//fmt.Println("SENDING: (Src,Dst) --> (",s.ServerId,",",msg.Pid,") (Message:",*msg,")")
				}
			}
		}
	}
}

/*	Receive the messages that come from the peer servers and queue them to the inbox channel
 */

func ReceiveMessages(ser servernode) {
	var e Envelope
	for {
		msg, err := ser.Serversocket.RecvMessage(0)
		check_err(err)
		a1 := strings.Split(msg[0], "&{")
		a2 := strings.Split(a1[1], " ")
		pid, err := strconv.Atoi(a2[0])
		check_err(err)
		mid, err := strconv.Atoi(a2[1])
		check_err(err)
		msgid := int64(mid)
		text := ""

		for b := 2; b < len(a2); b++ {
			text = text + " " + a2[b]
		}
		text1 := text[1:]
		msgtext := strings.Split(text1, "}")[0]

		e.Pid = pid
		e.MsgId = msgid
		e.Msg = msgtext
		go QueueInbox(ser, e, ser.Inbox())
	}
}

/*	Check for errors and panic out
 */

func check_err(e error) {
	if e != nil {
		panic(e)
	}
}

/*	Parse the given input cluster config file and collect all teh data for the cluster formation
 */

func ParseConfig(filename string) int {
	fconf, err := os.OpenFile(filename, os.O_RDONLY, 0600)
	check_err(err)
	rdconf := bufio.NewReader(fconf)
	nline, err := rdconf.ReadString('\n')
	index := 0

	for {
		if err != nil {
			break
		}

		line := strings.Split(nline, "\n")

		if !strings.Contains(nline, "#") && line[0] != "" {
			text := string(line[0])
			pair := strings.Split(text, ",")
			id := strings.TrimSpace(string(pair[0]))
			address := strings.TrimSpace(string(pair[1]))

			nodeid, err := strconv.Atoi(id)
			check_err(err)

			for k := 0; k < len(Peers); k++ {
				if Peers[k] == nodeid {
					fmt.Printf(strconv.Itoa(nodeid) + ": duplicate node IDs. Node IDs should be unique")
					return 1
				}
				for _, value := range PeerAddress {
					if value == address {
						fmt.Printf(address + ": duplicate IP:port number. IP:port should be unique for nodes")
						return 2
					}
				}
			}
			Peers = append(Peers, nodeid)
			peerIdIndex[nodeid] = index
			PeerAddress[nodeid] = address
			index = index + 1
		}
		nline, err = rdconf.ReadString('\n')
	}
	return 0
}
