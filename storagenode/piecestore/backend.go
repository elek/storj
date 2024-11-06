// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"encoding/binary"
	"hash"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/retain"
)

// PieceBackend is the minimal interface needed for the endpoints to do its job.
type PieceBackend interface {
	Writer(context.Context, storj.NodeID, storj.PieceID, pb.PieceHashAlgorithm, time.Time) (PieceWriter, error)
	Reader(context.Context, storj.NodeID, storj.PieceID) (PieceReader, error)
	StartRestore(context.Context, storj.NodeID) error
}

// PieceWriter is an interface for writing a piece.
type PieceWriter interface {
	io.Writer
	Size() int64
	Hash() []byte
	Cancel(context.Context) error
	Commit(context.Context, *pb.PieceHeader) error
}

// PieceReader is an interface for reading a piece.
type PieceReader interface {
	io.ReadSeekCloser
	Trash() bool
	Size() int64
	GetHashAndLimit(context.Context) (pb.PieceHash, pb.OrderLimit, error)
}

//
// hash store backend
//

// HashStoreBackend implements PieceBackend using the hashstore.
type HashStoreBackend struct {
	dir string
	bfm *retain.BloomFilterManager
	rtm *retain.RestoreTimeManager
	log *zap.Logger

	mu  sync.Mutex
	dbs map[storj.NodeID]*hashstore.DB
}

// NewHashStoreBackend constructs a new HashStoreBackend with the provided values. The log and hash
// directory are allowed to be the same.
func NewHashStoreBackend(
	dir string,
	bfm *retain.BloomFilterManager,
	rtm *retain.RestoreTimeManager,
	log *zap.Logger,
) *HashStoreBackend {
	return &HashStoreBackend{
		dir: dir,
		bfm: bfm,
		log: log,
		rtm: rtm,
		dbs: map[storj.NodeID]*hashstore.DB{},
	}
}

// Close closes the HashStoreBackend.
func (hsb *HashStoreBackend) Close() {
	hsb.mu.Lock()
	defer hsb.mu.Unlock()

	for _, db := range hsb.dbs {
		db.Close()
	}
}

// Stats implements monkit.StatSource.
func (hsb *HashStoreBackend) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	type IDDB struct {
		id storj.NodeID
		db *hashstore.DB
	}

	hsb.mu.Lock()
	dbs := make([]IDDB, 0, len(hsb.dbs))
	for id, db := range hsb.dbs {
		dbs = append(dbs, IDDB{id, db})
	}
	hsb.mu.Unlock()

	sort.Slice(dbs, func(i, j int) bool {
		return dbs[i].id.String() < dbs[j].id.String()
	})

	for _, iddb := range dbs {
		mon.Chain(monkit.StatSourceFromStruct(
			monkit.NewSeriesKey("hashstore").WithTag("satellite", iddb.id.String()),
			iddb.db.Stats(),
		))
	}
}

func (hsb *HashStoreBackend) getDB(satellite storj.NodeID) (*hashstore.DB, error) {
	hsb.mu.Lock()
	defer hsb.mu.Unlock()

	if db, exists := hsb.dbs[satellite]; exists {
		return db, nil
	}

	var log *zap.Logger
	if hsb.log != nil {
		log = hsb.log.With(zap.String("satellite", satellite.String()))
	}

	var shouldTrash func(context.Context, hashstore.Key, time.Time) bool
	var lastRestore func(context.Context) time.Time
	if hsb.bfm != nil {
		shouldTrash = hsb.bfm.GetBloomFilter(satellite)
	}
	if hsb.rtm != nil {
		lastRestore = func(ctx context.Context) time.Time {
			return hsb.rtm.GetRestoreTime(ctx, satellite, time.Now())
		}
	}

	db, err := hashstore.New(
		filepath.Join(hsb.dir, satellite.String()),
		log,
		shouldTrash,
		lastRestore,
	)
	if err != nil {
		return nil, err
	}

	hsb.dbs[satellite] = db

	return db, nil
}

// Writer implements PieceBackend.
func (hsb *HashStoreBackend) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, hash pb.PieceHashAlgorithm, expires time.Time) (PieceWriter, error) {
	db, err := hsb.getDB(satellite)
	if err != nil {
		return nil, err
	}
	writer, err := db.Create(ctx, pieceID, expires)
	if err != nil {
		return nil, err
	}
	return &hashStoreWriter{
		writer: writer,
		hasher: pb.NewHashFromAlgorithm(hash),
	}, nil
}

// Reader implements PieceBackend.
func (hsb *HashStoreBackend) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (PieceReader, error) {
	db, err := hsb.getDB(satellite)
	if err != nil {
		return nil, err
	}
	reader, err := db.Read(ctx, pieceID)
	if err != nil {
		return nil, err
	}
	return &hashStoreReader{
		sr:     io.NewSectionReader(reader, 0, reader.Size()-512),
		reader: reader,
	}, nil
}

// StartRestore implements PieceBackend.
func (hsb *HashStoreBackend) StartRestore(ctx context.Context, satellite storj.NodeID) error {
	if hsb.rtm == nil {
		return errs.New("Unsupported operation")
	}
	return hsb.rtm.SetRestoreTime(ctx, satellite, time.Now())
}

type hashStoreWriter struct {
	writer *hashstore.Writer
	size   int64

	hasher hash.Hash
}

func (hw *hashStoreWriter) Write(p []byte) (int, error) {
	n, err := hw.writer.Write(p)
	hw.size += int64(n)
	hw.hasher.Write(p[:n])
	return n, err
}

func (hw *hashStoreWriter) Size() int64                      { return hw.size }
func (hw *hashStoreWriter) Hash() []byte                     { return hw.hasher.Sum(nil) }
func (hw *hashStoreWriter) Cancel(ctx context.Context) error { hw.writer.Cancel(); return nil }

func (hw *hashStoreWriter) Commit(ctx context.Context, header *pb.PieceHeader) error {
	defer func() { _ = hw.Cancel(ctx) }()

	// marshal the header so we can put it as a footer.
	buf, err := pb.Marshal(header)
	if err != nil {
		return err
	} else if len(buf) > 512-2 {
		return errs.New("header too large")
	}

	// make a length prefixed footer and copy the header into it.
	var tmp [512]byte
	binary.BigEndian.PutUint16(tmp[0:2], uint16(len(buf)))
	copy(tmp[2:], buf)

	// write the footer.. header? footer.
	if _, err := hw.writer.Write(tmp[:]); err != nil {
		return err
	}

	// commit the piece.
	return hw.writer.Close()
}

type hashStoreReader struct {
	sr     *io.SectionReader
	reader *hashstore.Reader
}

func (hr *hashStoreReader) Read(p []byte) (int, error) { return hr.sr.Read(p) }
func (hr *hashStoreReader) Seek(offset int64, whence int) (int64, error) {
	return hr.sr.Seek(offset, whence)
}

func (hr *hashStoreReader) Close() error { return hr.reader.Close() }
func (hr *hashStoreReader) Trash() bool  { return hr.reader.Trash() }
func (hr *hashStoreReader) Size() int64  { return hr.reader.Size() - 512 }

func (hr *hashStoreReader) GetHashAndLimit(context.Context) (pb.PieceHash, pb.OrderLimit, error) {
	data, err := io.ReadAll(io.NewSectionReader(hr.reader, hr.reader.Size()-512, 512))
	if err != nil {
		return pb.PieceHash{}, pb.OrderLimit{}, err
	} else if len(data) != 512 {
		return pb.PieceHash{}, pb.OrderLimit{}, errs.New("footer too small")
	}
	l := binary.BigEndian.Uint16(data[0:2])
	if int(l) > len(data) {
		return pb.PieceHash{}, pb.OrderLimit{}, errs.New("footer length field too large: %d > %d", l, len(data))
	}
	var header pb.PieceHeader
	if err := pb.Unmarshal(data[:l], &header); err != nil {
		return pb.PieceHash{}, pb.OrderLimit{}, err
	}
	pieceHash := pb.PieceHash{
		PieceId:       hr.reader.Key(),
		Hash:          header.GetHash(),
		HashAlgorithm: header.GetHashAlgorithm(),
		PieceSize:     hr.Size(),
		Timestamp:     header.GetCreationTime(),
		Signature:     header.GetSignature(),
	}
	return pieceHash, header.OrderLimit, nil
}

//
// the old stuff
//

// OldPieceBackend takes a bunch of pieces the endpoint used and packages them into a PieceBackend.
type OldPieceBackend struct {
	store      *pieces.Store
	trashChore *pieces.TrashChore
	monitor    *monitor.Service
}

// NewOldPieceBackend constructs an OldPieceBackend.
func NewOldPieceBackend(store *pieces.Store, trashChore *pieces.TrashChore, monitor *monitor.Service) *OldPieceBackend {
	return &OldPieceBackend{
		store:      store,
		trashChore: trashChore,
		monitor:    monitor,
	}
}

// Writer implements PieceBackend and returns a PieceWriter for a piece.
func (opb *OldPieceBackend) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, hashAlgorithm pb.PieceHashAlgorithm, expiration time.Time) (PieceWriter, error) {
	writer, err := opb.store.Writer(ctx, satellite, pieceID, hashAlgorithm)
	if err != nil {
		return nil, err
	}
	return &oldPieceWriter{
		Writer:      writer,
		store:       opb.store,
		satelliteID: satellite,
		pieceID:     pieceID,
		expiration:  expiration,
	}, nil
}

// Reader implements PieceBackend and returns a PieceReader for a piece.
func (opb *OldPieceBackend) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (PieceReader, error) {
	reader, err := opb.store.Reader(ctx, satellite, pieceID)
	if err == nil {
		return &oldPieceReader{
			Reader:    reader,
			store:     opb.store,
			satellite: satellite,
			pieceID:   pieceID,
			trash:     false,
		}, nil
	}
	if !errs.Is(err, fs.ErrNotExist) {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	// check if the file is in trash, if so, restore it and
	// continue serving the download request.
	tryRestoreErr := opb.store.TryRestoreTrashPiece(ctx, satellite, pieceID)
	if tryRestoreErr != nil {
		opb.monitor.VerifyDirReadableLoop.TriggerWait()

		// we want to return the original "file does not exist" error to the rpc client
		return nil, rpcstatus.Wrap(rpcstatus.NotFound, err)
	}

	// try to open the file again
	reader, err = opb.store.Reader(ctx, satellite, pieceID)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}
	return &oldPieceReader{
		Reader:    reader,
		store:     opb.store,
		satellite: satellite,
		pieceID:   pieceID,
		trash:     true,
	}, nil
}

// StartRestore implements PieceBackend and starts a restore operation for a satellite.
func (opb *OldPieceBackend) StartRestore(ctx context.Context, satellite storj.NodeID) error {
	return opb.trashChore.StartRestore(ctx, satellite)
}

type oldPieceWriter struct {
	*pieces.Writer
	store       *pieces.Store
	satelliteID storj.NodeID
	pieceID     storj.PieceID
	expiration  time.Time
}

func (o *oldPieceWriter) Commit(ctx context.Context, header *pb.PieceHeader) error {
	if err := o.Writer.Commit(ctx, header); err != nil {
		return err
	}
	if !o.expiration.IsZero() {
		return o.store.SetExpiration(ctx, o.satelliteID, o.pieceID, o.expiration, o.Writer.Size())
	}
	return nil
}

type oldPieceReader struct {
	*pieces.Reader
	store     *pieces.Store
	satellite storj.NodeID
	pieceID   storj.PieceID
	trash     bool
}

func (o *oldPieceReader) Trash() bool { return o.trash }

func (o *oldPieceReader) GetHashAndLimit(ctx context.Context) (pb.PieceHash, pb.OrderLimit, error) {
	return o.store.GetHashAndLimit(ctx, o.satellite, o.pieceID, o.Reader)
}
