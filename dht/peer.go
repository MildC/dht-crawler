package dht

import "net"

type Peer interface {
	IP() net.IP
	Port() int
	Token() string
	CompactIPPortInfo() string
}

type peer struct {
	ip    net.IP
	port  int
	token string
}

func NewPeer(ip net.IP, port int, token string) Peer {
	return &peer{
		ip:    ip,
		port:  port,
		token: token,
	}
}

func NewPeerFromCompactIPPortInfo(compactInfo, token string) (Peer, error) {
	ip, port, err := decodeCompactIPPortInfo(compactInfo)
	if err != nil {
		return nil, err
	}

	return NewPeer(ip, port, token), nil
}

func (p *peer) IP() net.IP {
	return p.ip
}

func (p *peer) Port() int {
	return p.port
}

func (p *peer) Token() string {
	return p.token
}

// CompactIPPortInfo returns "Compact node info".
// See http://www.bittorrent.org/beps/bep_0005.html.
func (p *peer) CompactIPPortInfo() string {
	info, _ := encodeCompactIPPortInfo(p.ip, p.port)
	return info
}
