// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"fmt"

	"github.com/zeebo/xxh3"
)

const (
	rSize         = 64
	pSize         = 4096
	rPerP         = pSize / rSize
	_     uintptr = -(pSize % rSize) // ensure records evenly divide the page size
)

type page [pSize]byte

func (p *page) readRecord(n uint64, rec *Record) {
	if b := p[(n*rSize)%pSize:]; len(b) >= rSize {
		rec.read((*[rSize]byte)(b))
	}
}

func (p *page) writeRecord(n uint64, rec Record) {
	if b := p[(n*rSize)%pSize:]; len(b) >= rSize {
		rec.write((*[rSize]byte)(b))
	}
}

type expiration uint32

func newExpiration(t uint32, trash bool) expiration {
	if trash {
		return expiration(t<<1 | 1)
	}
	return expiration(t << 1)
}

func (e expiration) set() bool    { return e != 0 }
func (e expiration) trash() bool  { return e&1 == 1 }
func (e expiration) time() uint32 { return uint32(e >> 1) }

func maxExpiration(a, b expiration) expiration {
	if !a.set() || !b.set() {
		return 0
	}
	if a.trash() && !b.trash() { // if a is trash and b is not, keep a
		return a
	}
	if !a.trash() && b.trash() { // if b is trash and a is not, keep b
		return b
	}
	// they are the same, so pick the larger one.
	if a > b {
		return a
	}
	return b
}

type Record struct {
	Key      Key        // 256 bits (32b) of key
	Offset   uint64     // 48  bits (6b) of offset (256TB max file size)
	Log      uint64     // 64  bits (8b) of log id (effectively unlimited number of logs)
	Length   uint32     // 32  bits (4b) of length (4GB max piece size)
	Created  uint32     // 24  bits (3b) of days since epoch (~45900 years)
	Expires  expiration // 23  bits (3b) of days since epoch (~22900 years), 1 bit flag for trash
	Checksum uint64     // 63  bits (8b) of checksum, 1 bit flag reserved
}

func (r Record) String() string {
	return fmt.Sprintf(
		"{key:%v offset:%d log:%d length:%d created:%d expires:%d trash:%v checksum:%x}",
		r.Key, r.Offset, r.Log, r.Length, r.Created, r.Expires.time(), r.Expires.trash(), r.Checksum,
	)
}

func recordsEqualish(a, b Record) bool {
	a.Expires, a.Checksum = 0, 0
	b.Expires, b.Checksum = 0, 0
	return a == b
}

func (r *Record) index() uint64 { return keyIndex(&r.Key) }

func (r *Record) validChecksum() bool { return r.Checksum == r.computeChecksum() }
func (r *Record) setChecksum()        { r.Checksum = r.computeChecksum() }
func (r *Record) computeChecksum() uint64 {
	var buf [rSize]byte
	r.write(&buf)

	// reserve a bit of checksum space just in case we need a gross hacky flag in the future.
	return xxh3.Hash(buf[:56]) >> 1
}

func (r *Record) write(buf *[rSize]byte) {
	*(*Key)(buf[0:32]) = r.Key
	binary.LittleEndian.PutUint64(buf[32:32+8], r.Offset&0xffffffffffff)
	binary.LittleEndian.PutUint64(buf[38:38+8], r.Log&0xffffffffffffffff)
	binary.LittleEndian.PutUint32(buf[46:46+4], r.Length&0xffffffff)
	binary.LittleEndian.PutUint32(buf[50:50+4], r.Created&0xffffff)
	binary.LittleEndian.PutUint32(buf[53:53+4], uint32(r.Expires)&0xffffff)
	binary.LittleEndian.PutUint64(buf[56:56+8], r.Checksum&0xffffffffffffffff)
}

func (r *Record) read(buf *[rSize]byte) {
	r.Key = *(*Key)(buf[0:32])
	r.Offset = binary.LittleEndian.Uint64(buf[32:32+8]) & 0xffffffffffff
	r.Log = binary.LittleEndian.Uint64(buf[38:38+8]) & 0xffffffffffffffff
	r.Length = binary.LittleEndian.Uint32(buf[46:46+4]) & 0xffffffff
	r.Created = binary.LittleEndian.Uint32(buf[50:50+4]) & 0xffffff
	r.Expires = expiration(binary.LittleEndian.Uint32(buf[53:53+4]) & 0xffffff)
	r.Checksum = binary.LittleEndian.Uint64(buf[56:56+8]) & 0xffffffffffffffff
}
