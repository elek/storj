// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"errors"
	"log"
	"os"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"
)

// SpannerConfig includes all the configuration required by using spanner.
type SpannerConfig struct {
	Database string `help:"Database definition for spanner connection in the form  projects/P/instances/I/databases/DB"`
}

// SpannerAdapter implements Adapter for Google Spanner connections..
type SpannerAdapter struct {
	client *spanner.Client
}

// NewSpannerAdapter creates a new Spanner adapter.
func NewSpannerAdapter(ctx context.Context, cfg SpannerConfig) (*SpannerAdapter, error) {
	client, err := spanner.NewClientWithConfig(ctx, cfg.Database,
		spanner.ClientConfig{
			Logger:               log.New(os.Stdout, "spanner", log.LstdFlags),
			SessionPoolConfig:    spanner.DefaultSessionPoolConfig,
			DisableRouteToLeader: false})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &SpannerAdapter{
		client: client,
	}, nil
}

// TestingBeginObjectExactVersion implements Adapter.
func (s *SpannerAdapter) TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error {
	panic("implement me")
}

// GetObjectLastCommitted implements Adapter.
func (s *SpannerAdapter) GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted, object *Object) error {
	panic("implement me")
}

// BeginObjectNextVersion implements Adapter.
func (s *SpannerAdapter) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion, object *Object) error {
	_, err := s.client.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		enc, err := encryptionParameters{&opts.Encryption}.Value()
		if err != nil {
			return errs.Wrap(err)
		}

		lastVersionStmt := spanner.Statement{
			SQL: `SELECT version
					FROM objects
					WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
					ORDER BY version DESC
					LIMIT 1`,
			Params: map[string]interface{}{
				"project_id":  opts.ProjectID.Bytes(),
				"bucket_name": opts.BucketName,
				"object_key":  []byte(opts.ObjectKey),
			},
		}

		var nextVersion int64
		iter := txn.Query(ctx, lastVersionStmt)
		defer iter.Stop()
		for {
			row, err := iter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return errs.Wrap(err)
			}
			if err := row.Columns(&nextVersion); err != nil {
				return errs.Wrap(err)
			}
		}
		nextVersion++

		stmt := spanner.Statement{
			SQL: `INSERT objects (
					project_id, bucket_name, object_key, version, stream_id,
					expires_at, encryption,
					zombie_deletion_deadline,
					encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key)
				  VALUES(
                  	@project_id, @bucket_name,
					@object_key, @version, @stream_id, @expires_at,
					@encryption, @zombie_deletion_deadline,
					@encrypted_metadata, @encrypted_metadata_nonce, @encrypted_metadata_encrypted_key) 
                  THEN RETURN status,version,created_at`,
			Params: map[string]interface{}{
				"project_id":                       opts.ProjectID.Bytes(),
				"bucket_name":                      opts.BucketName,
				"object_key":                       []byte(opts.ObjectKey),
				"version":                          nextVersion,
				"stream_id":                        opts.StreamID.Bytes(),
				"expires_at":                       opts.ExpiresAt,
				"encryption":                       enc,
				"zombie_deletion_deadline":         opts.ZombieDeletionDeadline,
				"encrypted_metadata":               opts.EncryptedMetadata,
				"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
				"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
			},
		}
		updateIter := txn.Query(ctx, stmt)
		defer updateIter.Stop()
		for {
			row, err := updateIter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return errs.Wrap(err)
			}
			var status int64
			if err := row.Columns(&status, &object.Version, &object.CreatedAt); err != nil {
				return errs.Wrap(err)
			}
			object.Status = ObjectStatus(byte(status))
		}
		return nil
	})
	return err
}

// Close closes the internal client.
func (s *SpannerAdapter) Close() error {
	s.client.Close()
	return nil
}

var _ Adapter = &SpannerAdapter{}
