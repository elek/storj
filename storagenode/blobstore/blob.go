// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package blobstore

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// ErrInvalidBlobRef is returned when an blob reference is invalid.
var ErrInvalidBlobRef = errs.Class("invalid blob ref")

// FormatVersion represents differing storage format version values. Different Blobs implementors
// might interpret different FormatVersion values differently, but they share a type so that there
// can be a common StorageFormatVersion() call on the interface.
//
// Changes in FormatVersion might affect how a Blobs or BlobReader or BlobWriter instance works, or
// they might only be relevant to some higher layer. A FormatVersion must be specified when writing
// a new blob, and the blob storage interface must store that value with the blob somehow, so that
// the same FormatVersion is returned later when reading that stored blob.
type FormatVersion int

// BlobRef is a reference to a blob.
type BlobRef struct {
	Namespace []byte
	Key       []byte
}

// IsValid returns whether both namespace and key are specified.
func (ref *BlobRef) IsValid() bool {
	return len(ref.Namespace) > 0 && len(ref.Key) > 0
}

// BlobReader is an interface that groups Read, ReadAt, Seek and Close.
type BlobReader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
	// Size returns the size of the blob.
	Size() (int64, error)
	// StorageFormatVersion returns the storage format version associated with the blob.
	StorageFormatVersion() FormatVersion
}

// BlobWriter defines the interface that must be satisfied for a general blob storage provider.
// BlobWriter instances are returned by the Create() method on Blobs instances.
type BlobWriter interface {
	io.Writer
	io.Seeker
	// Cancel discards the blob.
	Cancel(context.Context) error
	// Commit ensures that the blob is readable by others.
	Commit(context.Context) error
	// Size returns the size of the blob
	Size() (int64, error)
	// StorageFormatVersion returns the storage format version associated with the blob.
	StorageFormatVersion() FormatVersion
}

// Blobs is a blob storage interface.
//
// architecture: Database
type Blobs interface {
	// Create creates a new blob that can be written.
	// Optionally takes a size argument for performance improvements, -1 is unknown size.
	Create(ctx context.Context, ref BlobRef, size int64) (BlobWriter, error)
	// Open opens a reader with the specified namespace and key.
	Open(ctx context.Context, ref BlobRef) (BlobReader, error)
	// OpenWithStorageFormat opens a reader for the already-located blob, avoiding the potential
	// need to check multiple storage formats to find the blob.
	OpenWithStorageFormat(ctx context.Context, ref BlobRef, formatVer FormatVersion) (BlobReader, error)
	// Delete deletes the blob with the namespace and key.
	Delete(ctx context.Context, ref BlobRef) error
	// DeleteWithStorageFormat deletes a blob of a specific storage format.
	DeleteWithStorageFormat(ctx context.Context, ref BlobRef, formatVer FormatVersion) error
	// DeleteNamespace deletes blobs folder for a specific namespace.
	DeleteNamespace(ctx context.Context, ref []byte) (err error)
	// DeleteTrashNamespace deletes the trash folder for a given namespace.
	DeleteTrashNamespace(ctx context.Context, namespace []byte) (err error)
	// Trash marks a file for pending deletion.
	Trash(ctx context.Context, ref BlobRef) error
	// RestoreTrash restores all files in the trash for a given namespace and returns the keys restored.
	RestoreTrash(ctx context.Context, namespace []byte) ([][]byte, error)
	// EmptyTrash removes all files in trash that were moved to trash prior to trashedBefore and returns the total bytes emptied and keys deleted.
	EmptyTrash(ctx context.Context, namespace []byte, trashedBefore time.Time) (int64, [][]byte, error)
	// TryRestoreTrashPiece attempts to restore a piece from the trash.
	// It returns nil if the piece was restored, or an error if the piece was not
	// in the trash or could not be restored.
	TryRestoreTrashPiece(ctx context.Context, ref BlobRef) error
	// Stat looks up disk metadata on the blob file.
	Stat(ctx context.Context, ref BlobRef) (BlobInfo, error)
	// StatWithStorageFormat looks up disk metadata for the blob file with the given storage format
	// version. This avoids the potential need to check multiple storage formats for the blob
	// when the format is already known.
	StatWithStorageFormat(ctx context.Context, ref BlobRef, formatVer FormatVersion) (BlobInfo, error)

	// FreeSpace return how much free space is left on the whole disk, not just the allocated disk space.
	FreeSpace(ctx context.Context) (int64, error)
	// SpaceUsedForTrash returns the total space used by the trash.
	SpaceUsedForTrash(ctx context.Context) (int64, error)
	// SpaceUsedForBlobs adds up how much is used in all namespaces.
	SpaceUsedForBlobs(ctx context.Context) (int64, error)
	// SpaceUsedForBlobsInNamespace adds up how much is used in the given namespace.
	SpaceUsedForBlobsInNamespace(ctx context.Context, namespace []byte) (int64, error)

	// ListNamespaces finds all namespaces in which keys might currently be stored.
	ListNamespaces(ctx context.Context) ([][]byte, error)
	// WalkNamespace executes walkFunc for each locally stored blob, stored with
	// storage format V1 or greater, in the given namespace. If walkFunc returns a non-nil
	// error, WalkNamespace will stop iterating and return the error immediately. The ctx
	// parameter is intended to allow canceling iteration early.
	WalkNamespace(ctx context.Context, namespace []byte, walkFunc func(BlobInfo) error) error

	// CheckWritability tests writability of the storage directory by creating and deleting a file.
	CheckWritability(ctx context.Context) error
	// CreateVerificationFile creates a file to be used for storage directory verification.
	CreateVerificationFile(ctx context.Context, id storj.NodeID) error
	// VerifyStorageDir verifies that the storage directory is correct by checking for the existence and validity
	// of the verification file.
	VerifyStorageDir(ctx context.Context, id storj.NodeID) error

	// Close closes the blob store and any resources associated with it.
	Close() error
}

// BlobInfo allows lazy inspection of a blob and its underlying file during iteration with
// WalkNamespace-type methods.
type BlobInfo interface {
	// BlobRef returns the relevant BlobRef for the blob.
	BlobRef() BlobRef
	// StorageFormatVersion indicates the storage format version used to store the piece.
	StorageFormatVersion() FormatVersion
	// FullPath gives the full path to the on-disk blob file.
	FullPath(ctx context.Context) (string, error)
	// Stat does a stat on the on-disk blob file.
	Stat(ctx context.Context) (os.FileInfo, error)
}
