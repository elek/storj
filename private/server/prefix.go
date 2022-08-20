package server

import (
	"bytes"
	"io"
	"net"
)

type prefixConn struct {
	io.Reader
	net.Conn
}

func newPrefixConn(data []byte, conn net.Conn) *prefixConn {
	return &prefixConn{
		Reader: io.MultiReader(bytes.NewReader(data), conn),
		Conn:   conn,
	}
}

func (pc *prefixConn) Read(p []byte) (n int, err error) {
	return pc.Reader.Read(p)
}

type PrefixedListener struct {
	net.Listener
	prefix []byte
}

func NewPrefixedListener(prefix []byte, listener net.Listener) net.Listener {
	return &PrefixedListener{
		Listener: listener,
		prefix:   prefix,
	}
}
func (p *PrefixedListener) Accept() (net.Conn, error) {
	conn, err := p.Listener.Accept()
	if err != nil {
		return conn, err
	}
	return newPrefixConn(p.prefix, conn), nil
}

var _ net.Listener = &PrefixedListener{}
