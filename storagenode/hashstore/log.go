// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"io"
	"math"
	"os"
	"sync"

	"github.com/zeebo/errs"
)

type logFile struct {
	// immutable fields
	fh *os.File
	id uint64

	// mutable but unsynchronized fields
	size uint64

	// mutable and synchronized fields
	mu      sync.Mutex // protects the following fields
	refs    uint32     // refcount of acquired handles to the log file
	close   bool       // intent to close the file when refs == 0
	closed  flag       // set when the file has been closed
	removed flag       // set when the file has been removed
}

func newLogFile(fh *os.File, id uint64, size uint64) *logFile {
	return &logFile{
		fh:   fh,
		id:   id,
		size: size,
	}
}

func (l *logFile) performIntents() {
	if l.refs != 0 {
		return
	}
	if l.close && !l.closed.set() {
		_ = l.fh.Close()
	}
}

func (l *logFile) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.close = true
	l.performIntents()
}

func (l *logFile) Remove() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.removed.set() {
		_ = os.Remove(l.fh.Name())
	}
}

func (l *logFile) Acquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.close {
		return false
	}

	l.refs++
	return true
}

func (l *logFile) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refs--
	l.performIntents()
}

//
// heap of log files by size
//

type logHeap []*logFile

func (h logHeap) Len() int           { return len(h) }
func (h logHeap) Less(i, j int) bool { return h[i].size > h[j].size }
func (h logHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *logHeap) Push(x any)        { *h = append(*h, x.(*logFile)) }
func (h *logHeap) Pop() any {
	n := len(*h)
	x := (*h)[n-1]
	*h = (*h)[:n-1]
	return x
}

//
// Reader
//

// Reader is a type that reads a section from a log file.
type Reader struct {
	r   *io.SectionReader
	lf  *logFile
	rec record
}

func newLogReader(lf *logFile, rec record) *Reader {
	return &Reader{
		r:   io.NewSectionReader(lf.fh, int64(rec.offset), int64(rec.length)),
		lf:  lf,
		rec: rec,
	}
}

// Key returns the key of thereader.
func (l *Reader) Key() Key { return l.rec.key }

// Size returns the size of the reader.
func (l *Reader) Size() int64 { return int64(l.rec.length) }

// Trash returns true if the reader was for a trashed piece.
func (l *Reader) Trash() bool { return l.rec.expires.trash() }

// Seek implements io.Seeker.
func (l *Reader) Seek(offset int64, whence int) (int64, error) { return l.r.Seek(offset, whence) }

// ReadAt implements io.ReaderAt.
func (l *Reader) ReadAt(p []byte, off int64) (int, error) { return l.r.ReadAt(p, off) }

// Read implements io.Reader.
func (l *Reader) Read(p []byte) (int, error) { return l.r.Read(p) }

// Release returns the resources associated with the reader. It must be called when done.
func (l *Reader) Release() { l.lf.Release() }

// Close is like Release but implements io.Closer. The returned error is always nil.
func (l *Reader) Close() error { l.lf.Release(); return nil }

//
// Writer
//

// Writer is a type that allows one to write a piece to a log file.
type Writer struct {
	store *store
	lf    *logFile

	mu       sync.Mutex // protects the following fields
	canceled flag
	closed   flag
	rec      record
}

func newWriter(store *store, lf *logFile, rec record) *Writer {
	return &Writer{
		store: store,
		lf:    lf,

		rec: rec,
	}
}

// Size returns the number of bytes written to the Writer.
func (h *Writer) Size() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	return int64(h.rec.length)
}

// Close commits the writes that have happened. Close or Cancel must be called at least once.
func (h *Writer) Close() error {
	// if we are not the first to close or we are canceled, do nothing.
	h.mu.Lock()
	if h.closed.set() || h.canceled.get() {
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()

	// always replace the log file when done.
	defer h.store.replaceLogFile(h.lf)

	// we're about to write rSize bytes. if we can align the file to 4k after writing the record by
	// writing less than 64 bytes, try to do so. we do this write separately from appending the
	// record because otherwise we would have to allocate a variable width buffer causing an
	// allocation on every Close instead of just on the calls that fix alignment.
	var written int
	if align := 4096 - ((uint64(h.rec.length) + h.lf.size + rSize) % 4096); align > 0 && align < 64 {
		written, _ = h.lf.fh.Write(make([]byte, align))
	}

	// append the record to the log file for reconstruction.
	var buf [rSize]byte
	h.rec.setChecksum()
	h.rec.write(&buf)

	if _, err := h.lf.fh.Write(buf[:]); err != nil {
		// if we can't write the entry, we should abort the write operation so that we can always
		// reconstruct the table from the log file. attempt to reclaim space by seeking backwards
		// to the record offset.
		_, _ = h.lf.fh.Seek(int64(h.lf.size), io.SeekStart)
		return errs.Wrap(err)
	}

	// increase our in-memory estimate of the size of the log file for sorting.
	h.lf.size += uint64(h.rec.length) + uint64(written) + rSize

	return h.store.addRecord(h.rec)
}

// Cancel discards the writes that have happened. Close or Cancel must be called at least once.
func (h *Writer) Cancel() {
	// if we are not the first to cancel or we are closed, do nothing.
	h.mu.Lock()
	if h.canceled.set() || h.closed.get() {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()

	// always replace the log file when done.
	defer h.store.replaceLogFile(h.lf)

	// attempt to seek backwards the amount we have written to reclaim space.
	if h.rec.length != 0 {
		_, _ = h.lf.fh.Seek(-int64(h.rec.length), io.SeekCurrent)
	}
}

// Write implements io.Writer.
func (h *Writer) Write(p []byte) (n int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.canceled || h.closed {
		return 0, errs.New("invalid handle")
	} else if uint64(h.rec.length)+uint64(len(p)) > math.MaxUint32 {
		return 0, errs.New("piece too large")
	}

	n, err = h.lf.fh.Write(p)
	h.rec.length += uint32(n)

	return n, err
}
