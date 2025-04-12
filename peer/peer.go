// Package peer provides functionalities around handling and synching peers on the network.
package peer

import "sync"

// Peer represents a node on the network.
type Peer struct {
	Host string
}

func New(host string) Peer {
	return Peer{Host: host}
}

// Match validates if the host is the same.
func (p Peer) Match(host string) bool {
	return p.Host == host
}

// PeerStatus represents information about any peer.
type PeerStatus struct {
	LatestBlockHash   string `json:"latest_block_hash"`
	LatestBlockNumber uint64 `json:"latest_block_number"`
	KnownPeers        []Peer `json:"known_peers"`
}

// PeerSet represents a set of known peers.
type PeerSet struct {
	mu  sync.RWMutex
	set map[Peer]struct{}
}

func NewPeerSet() *PeerSet {
	return &PeerSet{
		set: make(map[Peer]struct{}),
	}
}

func (ps *PeerSet) Add(peer Peer) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	_, exists := ps.set[peer]
	if exists {
		return false
	}

	ps.set[peer] = struct{}{}
	return true
}

func (ps *PeerSet) Remove(peer Peer) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.set, peer)
}

// Copy returns a list of known peers
func (ps *PeerSet) Copy(host string) []Peer {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peers := make([]Peer, 0, len(ps.set))
	for peer := range ps.set {
		if !peer.Match(host) {
			peers = append(peers, peer)
		}
	}
	return peers
}
