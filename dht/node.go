package dht

import (
	"errors"
	"net"
	"strings"
	"time"
)

// Node represents a DHT node.
type Node interface {
	ID() *bitmap
	IDRawString() string
	Address() *net.UDPAddr
	LastActiveTime() time.Time
	CompactIPPortInfo() string
	CompactNodeInfo() string
}

type node struct {
	id             *bitmap
	address        *net.UDPAddr
	lastActiveTime time.Time
}

func NewNode(id string, address *net.UDPAddr) Node {
	return &node{newBitmapFromString(id), address, time.Now()}
}

func NewNodeNetworkAddress(id, network, address string) (Node, error) {
	if len(id) != 20 {
		return nil, errors.New("node id should be a 20-length string")
	}

	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}

	return &node{newBitmapFromString(id), addr, time.Now()}, nil
}

// NewNodeFromCompactInfo parses compactNodeInfo and returns a node pointer.
func NewNodeFromCompactInfo(compactNodeInfo string, network string) (Node, error) {
	if len(compactNodeInfo) != 26 {
		return nil, errors.New("compactNodeInfo should be a 26-length string")
	}

	id := compactNodeInfo[:20]
	ip, port, _ := decodeCompactIPPortInfo(compactNodeInfo[20:])

	return NewNodeNetworkAddress(id, network, genAddress(ip.String(), port))
}

func (n *node) ID() *bitmap {
	return n.id
}

func (n *node) IDRawString() string {
	return n.id.RawString()
}

func (n *node) Address() *net.UDPAddr {
	return n.address
}

func (n *node) LastActiveTime() time.Time {
	return n.lastActiveTime
}

// CompactIPPortInfo returns "Compact IP-address/port info".
// See http://www.bittorrent.org/beps/bep_0005.html.
func (n *node) CompactIPPortInfo() string {
	info, _ := encodeCompactIPPortInfo(n.address.IP, n.address.Port)
	return info
}

// CompactNodeInfo returns "Compact node info".
// See http://www.bittorrent.org/beps/bep_0005.html.
func (n *node) CompactNodeInfo() string {
	return strings.Join([]string{
		n.id.RawString(), n.CompactIPPortInfo(),
	}, "")
}
