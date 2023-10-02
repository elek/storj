package common

import (
	"context"
	"sync"
)

type ClosablePeer struct {
	Peer Peer

	Ctx         context.Context
	Cancel      func()
	RunFinished chan struct{} // it is closed after peer.Run returns

	close sync.Once
	err   error
}

func NewClosablePeer(peer Peer) ClosablePeer {
	return ClosablePeer{
		Peer:        peer,
		RunFinished: make(chan struct{}),
	}
}

// Close closes safely the peer.
func (peer *ClosablePeer) Close() error {
	peer.Cancel()

	peer.close.Do(func() {
		<-peer.RunFinished // wait for Run to complete
		peer.err = peer.Peer.Close()
	})

	return peer.err
}
