package raftwrapper

import (
	"fmt"
	"strconv"
	"strings"
)

type Peer struct {
	ID       string
	Host     string
	RaftPort uint16
}

func NewPeer(id string, raftAddr string) *Peer {

	addrParts := strings.Split(raftAddr, ":")
	if len(addrParts) != 2 {
		panic(fmt.Errorf("cant parse raft addr '%s'", raftAddr))
	}
	host := addrParts[0]
	raftPort, _ := strconv.ParseUint(addrParts[1], 10, 16)

	return &Peer{
		ID:       id,
		Host:     host,
		RaftPort: uint16(raftPort),
	}
}

func (p Peer) IsReady() bool {
	return true
}
