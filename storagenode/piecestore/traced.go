package piecestore

import (
	"context"
	"github.com/spacemonkeygo/monkit/v3"
	"storj.io/drpc"
)

type TracedStream struct {
	stream drpc.Stream
	rpc    string
}

func (t *TracedStream) Context() context.Context {
	return t.stream.Context()
}

func (t *TracedStream) MsgSend(msg drpc.Message, enc drpc.Encoding) (err error) {
	ctx := t.stream.Context()
	defer mon.Task(monkit.NewSeriesTag("rpc", t.rpc))(&ctx)(&err)
	return t.stream.MsgSend(msg, enc)
}

func (t *TracedStream) MsgRecv(msg drpc.Message, enc drpc.Encoding) (err error) {
	ctx := t.stream.Context()
	defer mon.Task(monkit.NewSeriesTag("rpc", t.rpc))(&ctx)(&err)
	return t.stream.MsgRecv(msg, enc)
}

func (t *TracedStream) CloseSend() error {
	return t.stream.CloseSend()
}

func (t *TracedStream) Close() error {
	return t.stream.Close()
}

var _ drpc.Stream = &TracedStream{}
