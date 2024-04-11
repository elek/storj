// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"

	"storj.io/common/dbutil/pgxutil"
	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// RawObject defines the full object that is stored in the database. It should be rarely used directly.
type RawObject struct {
	ObjectStream

	CreatedAt time.Time
	ExpiresAt *time.Time

	Status       ObjectStatus
	SegmentCount int32

	EncryptedMetadataNonce        []byte
	EncryptedMetadata             []byte
	EncryptedMetadataEncryptedKey []byte

	// TotalPlainSize is 0 for a migrated object.
	TotalPlainSize     int64
	TotalEncryptedSize int64
	// FixedSegmentSize is 0 for a migrated object.
	FixedSegmentSize int32

	Encryption storj.EncryptionParameters

	// ZombieDeletionDeadline defines when the pending raw object should be deleted from the database.
	// This is as a safeguard against objects that failed to upload and the client has not indicated
	// whether they want to continue uploading or delete the already uploaded data.
	ZombieDeletionDeadline *time.Time
}

// RawSegment defines the full segment that is stored in the database. It should be rarely used directly.
type RawSegment struct {
	StreamID uuid.UUID
	Position SegmentPosition

	CreatedAt  time.Time // non-nillable
	RepairedAt *time.Time
	ExpiresAt  *time.Time

	RootPieceID       storj.PieceID
	EncryptedKeyNonce []byte
	EncryptedKey      []byte

	EncryptedSize int32 // size of the whole segment (not a piece)
	// PlainSize is 0 for a migrated object.
	PlainSize int32
	// PlainOffset is 0 for a migrated object.
	PlainOffset   int64
	EncryptedETag []byte

	Redundancy storj.RedundancyScheme

	InlineData []byte
	Pieces     Pieces
	// TODO remove this
	AliasPieces AliasPieces

	Placement storj.PlacementConstraint
}

// RawCopy contains a copy that is stored in the database.
type RawCopy struct {
	StreamID         uuid.UUID
	AncestorStreamID uuid.UUID
}

// RawState contains full state of a table.
type RawState struct {
	Objects  []RawObject
	Segments []RawSegment
}

// TestingGetState returns the state of the database.
func (db *DB) TestingGetState(ctx context.Context) (_ *RawState, err error) {
	state := &RawState{}

	state.Objects, err = db.testingGetAllObjects(ctx)
	if err != nil {
		return nil, Error.New("GetState: %w", err)
	}

	state.Segments, err = db.testingGetAllSegments(ctx)
	if err != nil {
		return nil, Error.New("GetState: %w", err)
	}

	return state, nil
}

// TestingDeleteAll deletes all objects and segments from the database.
func (db *DB) TestingDeleteAll(ctx context.Context) (err error) {
	db.aliasCache.reset()
	return db.ChooseAdapter(uuid.UUID{}).TestingDeleteAll(ctx)
}

func (p *PostgresAdapter) TestingDeleteAll(ctx context.Context) (err error) {
	_, err = p.db.ExecContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM objects;
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM segments;
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM node_aliases;
		WITH ignore_full_scan_for_test AS (SELECT 1) SELECT setval('node_alias_seq', 1, false);
	`)
	return Error.Wrap(err)
}

func (s *SpannerAdapter) TestingDeleteAll(ctx context.Context) (err error) {
	_, err = s.client.Apply(ctx, []*spanner.Mutation{
		spanner.Delete("objects", spanner.AllKeys()),
		spanner.Delete("segments", spanner.AllKeys()),
		spanner.Delete("node_aliases", spanner.AllKeys()),
	})
	return Error.Wrap(err)
}

// testingGetAllObjects returns the state of the database.
func (db *DB) testingGetAllObjects(ctx context.Context) (_ []RawObject, err error) {
	if db.db == nil {
		return []RawObject{}, nil
	}

	objs := []RawObject{}

	rows, err := db.db.QueryContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1)
		SELECT
			project_id, bucket_name, object_key, version, stream_id,
			created_at, expires_at,
			status, segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption,
			zombie_deletion_deadline
		FROM objects
		ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version ASC
	`)
	if err != nil {
		return nil, Error.New("testingGetAllObjects query: %w", err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		var obj RawObject
		err := rows.Scan(
			&obj.ProjectID,
			&obj.BucketName,
			&obj.ObjectKey,
			&obj.Version,
			&obj.StreamID,

			&obj.CreatedAt,
			&obj.ExpiresAt,

			&obj.Status, // TODO: fix encoding
			&obj.SegmentCount,

			&obj.EncryptedMetadataNonce,
			&obj.EncryptedMetadata,
			&obj.EncryptedMetadataEncryptedKey,

			&obj.TotalPlainSize,
			&obj.TotalEncryptedSize,
			&obj.FixedSegmentSize,

			encryptionParameters{&obj.Encryption},
			&obj.ZombieDeletionDeadline,
		)
		if err != nil {
			return nil, Error.New("testingGetAllObjects scan failed: %w", err)
		}
		objs = append(objs, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, Error.New("testingGetAllObjects scan failed: %w", err)
	}

	if len(objs) == 0 {
		return nil, nil
	}
	return objs, nil
}

// TestingBatchInsertObjects batch inserts objects for testing.
// This implementation does no verification on the correctness of objects.
func (db *DB) TestingBatchInsertObjects(ctx context.Context, objects []RawObject) (err error) {
	const maxRowsPerCopy = 250000

	return Error.Wrap(pgxutil.Conn(ctx, db.db,
		func(conn *pgx.Conn) error {
			progress, total := 0, len(objects)
			for len(objects) > 0 {
				batch := objects
				if len(batch) > maxRowsPerCopy {
					batch = batch[:maxRowsPerCopy]
				}
				objects = objects[len(batch):]

				source := newCopyFromRawObjects(batch)
				_, err := conn.CopyFrom(ctx, pgx.Identifier{"objects"}, source.Columns(), source)
				if err != nil {
					return err
				}

				progress += len(batch)
				db.log.Info("batch insert", zap.Int("progress", progress), zap.Int("total", total))
			}
			return err
		}))
}

type copyFromRawObjects struct {
	idx  int
	rows []RawObject
	row  []any
}

func newCopyFromRawObjects(rows []RawObject) *copyFromRawObjects {
	return &copyFromRawObjects{
		rows: rows,
		idx:  -1,
	}
}

func (ctr *copyFromRawObjects) Next() bool {
	ctr.idx++
	return ctr.idx < len(ctr.rows)
}

func (ctr *copyFromRawObjects) Columns() []string {
	return []string{
		"project_id",
		"bucket_name",
		"object_key",
		"version",
		"stream_id",

		"created_at",
		"expires_at",

		"status",
		"segment_count",

		"encrypted_metadata_nonce",
		"encrypted_metadata",
		"encrypted_metadata_encrypted_key",

		"total_plain_size",
		"total_encrypted_size",
		"fixed_segment_size",

		"encryption",
		"zombie_deletion_deadline",
	}
}

func (ctr *copyFromRawObjects) Values() ([]any, error) {
	obj := &ctr.rows[ctr.idx]
	ctr.row = append(ctr.row[:0],
		obj.ProjectID.Bytes(),
		[]byte(obj.BucketName),
		[]byte(obj.ObjectKey),
		obj.Version,
		obj.StreamID.Bytes(),

		obj.CreatedAt,
		obj.ExpiresAt,

		obj.Status, // TODO: fix encoding
		obj.SegmentCount,

		obj.EncryptedMetadataNonce,
		obj.EncryptedMetadata,
		obj.EncryptedMetadataEncryptedKey,

		obj.TotalPlainSize,
		obj.TotalEncryptedSize,
		obj.FixedSegmentSize,

		encryptionParameters{&obj.Encryption},
		obj.ZombieDeletionDeadline,
	)
	return ctr.row, nil
}

func (ctr *copyFromRawObjects) Err() error { return nil }

// testingGetAllSegments returns the state of the database.
func (db *DB) testingGetAllSegments(ctx context.Context) (_ []RawSegment, err error) {
	return db.ChooseAdapter(uuid.UUID{}).TestingGetAllSegments(ctx, db.aliasCache)
}

// TestingGetAllSegments returns the state of the database.
func (p *PostgresAdapter) TestingGetAllSegments(ctx context.Context, aliasCache *NodeAliasCache) (_ []RawSegment, err error) {
	segs := []RawSegment{}

	rows, err := p.db.QueryContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1)
		SELECT
			stream_id, position,
			created_at, repaired_at, expires_at,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size,
			plain_offset, plain_size,
			encrypted_etag,
			redundancy,
			inline_data, remote_alias_pieces,
			placement
		FROM segments
		ORDER BY stream_id ASC, position ASC
	`)
	if err != nil {
		return nil, Error.New("testingGetAllSegments query: %w", err)
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		var seg RawSegment
		err := rows.Scan(
			&seg.StreamID,
			&seg.Position,

			&seg.CreatedAt,
			&seg.RepairedAt,
			&seg.ExpiresAt,

			&seg.RootPieceID,
			&seg.EncryptedKeyNonce,
			&seg.EncryptedKey,

			&seg.EncryptedSize,
			&seg.PlainOffset,
			&seg.PlainSize,
			&seg.EncryptedETag,

			redundancyScheme{&seg.Redundancy},

			&seg.InlineData,
			&seg.AliasPieces,
			&seg.Placement,
		)
		if err != nil {
			return nil, Error.New("testingGetAllSegments scan failed: %w", err)
		}

		seg.Pieces, err = aliasCache.ConvertAliasesToPieces(ctx, seg.AliasPieces)
		if err != nil {
			return nil, Error.New("testingGetAllSegments convert aliases to pieces failed: %w", err)
		}

		segs = append(segs, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, Error.New("testingGetAllSegments scan failed: %w", err)
	}

	if len(segs) == 0 {
		return nil, nil
	}
	return segs, nil
}

func (p *SpannerAdapter) TestingGetAllSegments(ctx context.Context, aliasCache *NodeAliasCache) (segments []RawSegment, err error) {
	stmt := spanner.Statement{SQL: `
		SELECT
			stream_id, position,
			created_at, repaired_at, expires_at,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size, plain_offset, plain_size,
			encrypted_etag,
			redundancy,
			inline_data, remote_alias_pieces,
			placement
		FROM segments
		ORDER BY stream_id ASC, position ASC
	`}
	iter := p.client.Single().Query(ctx, stmt)
	defer iter.Stop()

	for {
		row, err := iter.Next()
		if err == iterator.Done {
			return segments, nil
		}
		if err != nil {
			return nil, err
		}

		var position int64
		var createdAt time.Time
		var repairedAt, expiresAt spanner.NullTime
		var encryptedSize, plainOffset, plainSize, redundancy, placement int64
		var streamID, rootPieceID, encryptedKeyNonce, encryptedKey, encryptedETag, inlineData, remoteAliasPieces []byte
		if err := row.Columns(&streamID, &position,
			&createdAt, &repairedAt, &expiresAt,
			&rootPieceID, &encryptedKeyNonce, &encryptedKey,
			&encryptedSize, &plainOffset, &plainSize,
			&encryptedETag,
			&redundancy,
			&inlineData, &remoteAliasPieces,
			&placement,
		); err != nil {
			return nil, err
		}
		var segment RawSegment
		segment.StreamID, err = uuid.FromBytes([]byte(streamID))
		if err != nil {
			return nil, err
		}
		segment.Position = SegmentPositionFromEncoded(uint64(position))
		segment.CreatedAt = createdAt
		segment.RootPieceID, err = storj.PieceIDFromBytes(rootPieceID)
		if err != nil {
			return nil, err
		}
		segment.EncryptedKeyNonce = encryptedKeyNonce
		segment.EncryptedKey = encryptedKey
		if repairedAt.Valid {
			segment.RepairedAt = &repairedAt.Time
		}
		if expiresAt.Valid {
			segment.ExpiresAt = &expiresAt.Time
		}
		segment.EncryptedSize = int32(encryptedSize)
		segment.PlainOffset = plainOffset
		segment.PlainSize = int32(plainSize)
		segment.EncryptedETag = encryptedETag
		rs := redundancyScheme{RedundancyScheme: &storj.RedundancyScheme{}}
		rs.Scan(redundancy)
		segment.Redundancy = *rs.RedundancyScheme
		segment.InlineData = inlineData

		aliasPieces := AliasPieces{}
		err = aliasPieces.SetBytes(remoteAliasPieces)
		if err != nil {
			return nil, err
		}
		segment.Placement = storj.PlacementConstraint(placement)

		segment.Pieces, err = aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
		if err != nil {
			return nil, Error.New("testingGetAllSegments convert aliases to pieces failed: %w", err)
		}

		segments = append(segments, segment)
	}
}

// TestingBatchInsertSegments batch inserts segments for testing.
// This implementation does no verification on the correctness of segments.
func (db *DB) TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) (err error) {
	return db.ChooseAdapter(uuid.UUID{}).TestingBatchInsertSegments(ctx, aliasCache, segments)
}

// TestingBatchInsertSegments implements postgres adapter.
func (p *PostgresAdapter) TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) (err error) {
	const maxRowsPerCopy = 250000

	minLength := len(segments)
	if maxRowsPerCopy < minLength {
		minLength = maxRowsPerCopy
	}

	aliases := make([]AliasPieces, 0, minLength)
	return Error.Wrap(pgxutil.Conn(ctx, p.db,
		func(conn *pgx.Conn) error {
			progress, total := 0, len(segments)
			for len(segments) > 0 {
				batch := segments
				if len(batch) > maxRowsPerCopy {
					batch = batch[:maxRowsPerCopy]
				}
				segments = segments[len(batch):]

				aliases = aliases[:len(batch)]
				for i, segment := range batch {
					aliases[i], err = aliasCache.EnsurePiecesToAliases(ctx, segment.Pieces)
					if err != nil {
						return err
					}
				}

				source := newCopyFromRawSegments(batch, aliases)
				_, err := conn.CopyFrom(ctx, pgx.Identifier{"segments"}, source.Columns(), source)
				if err != nil {
					return err
				}

				progress += len(batch)
				p.log.Info("batch insert", zap.Int("progress", progress), zap.Int("total", total))
			}
			return err
		}))
}

type copyFromRawSegments struct {
	idx     int
	rows    []RawSegment
	aliases []AliasPieces
	row     []any
}

func newCopyFromRawSegments(rows []RawSegment, aliases []AliasPieces) *copyFromRawSegments {
	return &copyFromRawSegments{
		rows:    rows,
		aliases: aliases,
		idx:     -1,
	}
}

func (ctr *copyFromRawSegments) Next() bool {
	ctr.idx++
	return ctr.idx < len(ctr.rows)
}

func (ctr *copyFromRawSegments) Columns() []string {
	return []string{
		"stream_id",
		"position",

		"created_at",
		"repaired_at",
		"expires_at",

		"root_piece_id",
		"encrypted_key_nonce",
		"encrypted_key",
		"encrypted_etag",

		"encrypted_size",
		"plain_size",
		"plain_offset",

		"redundancy",
		"inline_data",
		"remote_alias_pieces",
		"placement",
	}
}

func (ctr *copyFromRawSegments) Values() ([]any, error) {
	obj := &ctr.rows[ctr.idx]
	aliases := &ctr.aliases[ctr.idx]

	aliasPieces, err := aliases.Bytes()
	if err != nil {
		return nil, err
	}
	ctr.row = append(ctr.row[:0],
		obj.StreamID.Bytes(),
		obj.Position.Encode(),

		obj.CreatedAt,
		obj.RepairedAt,
		obj.ExpiresAt,

		obj.RootPieceID.Bytes(),
		obj.EncryptedKeyNonce,
		obj.EncryptedKey,
		obj.EncryptedETag,

		obj.EncryptedSize,
		obj.PlainSize,
		obj.PlainOffset,

		redundancyScheme{&obj.Redundancy},
		obj.InlineData,
		aliasPieces,
		obj.Placement,
	)
	return ctr.row, nil
}

func (ctr *copyFromRawSegments) Err() error { return nil }

func (s *SpannerAdapter) TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) (err error) {
	columns := []string{
		"stream_id",
		"position",

		"created_at",
		"repaired_at",
		"expires_at",

		"root_piece_id",
		"encrypted_key_nonce",
		"encrypted_key",
		"encrypted_etag",

		"encrypted_size",
		"plain_size",
		"plain_offset",

		"redundancy",
		"inline_data",
		"remote_alias_pieces",
		"placement",
	}

	mutations := make([]*spanner.Mutation, len(segments))
	for i, segment := range segments {
		segment.AliasPieces, err = aliasCache.EnsurePiecesToAliases(ctx, segment.Pieces)
		if err != nil {
			return errs.Wrap(err)
		}
		aliasPieces, err := segment.AliasPieces.Bytes()
		if err != nil {
			return errs.Wrap(err)
		}
		redundancyValue, err := redundancyScheme{&segment.Redundancy}.Value()
		if err != nil {
			return errs.Wrap(err)
		}

		redundancy := redundancyValue.(int64)

		vals := append([]interface{}{},
			segment.StreamID.Bytes(),
			int64(segment.Position.Encode()), // TODO

			segment.CreatedAt,
			segment.RepairedAt,
			segment.ExpiresAt,

			segment.RootPieceID.Bytes(),
			segment.EncryptedKeyNonce,
			segment.EncryptedKey,
			segment.EncryptedETag,

			int64(segment.EncryptedSize),
			int64(segment.PlainSize),
			segment.PlainOffset,

			redundancy,
			segment.InlineData,
			aliasPieces,
			int64(segment.Placement),
		)

		mutations[i] = spanner.InsertOrUpdate("segments", columns, vals)
	}

	_, err = s.client.Apply(ctx, mutations)
	return errs.Wrap(err)
}
