package perf

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"os"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storage/sqlite"
	"storj.io/storj/storagenode/pieces"
	"testing"
	"time"
)

type factory func(b *testing.B, name string, ctx *testcontext.Context, sync string) storage.Blobs

func BenchmarkBulkWrite(b *testing.B) {
	ctx := testcontext.NewWithContextAndTimeout(context.Background(), b, 1*time.Hour)
	defer ctx.Cleanup()

	// setup test parameters
	const blockSize = int(256 * memory.KiB)
	satelliteID := testrand.NodeID()
	source := testrand.Bytes(2319872)

	for _, db := range []string{"file", "sqlite"} {
		for _, hashAlgo := range []pb.PieceHashAlgorithm{pb.PieceHashAlgorithm_SHA256, pb.PieceHashAlgorithm_BLAKE3} {
			for _, sync := range []string{"sync", "nosync"} {
				b.Run(fmt.Sprintf("%s-%s-%s", db, sync, hashAlgo.String()), func(b *testing.B) {

					var f factory
					switch db {
					case "file":
						f = createFileBlob

					case "sqlite":
						f = createSqliteBlob
					}
					blobs := f(b, sync+"-"+hashAlgo.String(), ctx, sync)
					defer ctx.Check(blobs.Close)
					store := pieces.NewStore(zap.NewNop(), blobs, nil, nil, nil, pieces.DefaultConfig)

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						pieceID := testrand.PieceID()
						writer, err := store.Writer(ctx, satelliteID, pieceID, hashAlgo)
						require.NoError(b, err)

						data := source
						for len(data) > 0 {
							n := blockSize
							if n > len(data) {
								n = len(data)
							}
							_, err = writer.Write(data[:n])
							require.NoError(b, err)
							data = data[n:]
						}
						b.SetBytes(int64(len(source)))
						require.NoError(b, writer.Commit(ctx, &pb.PieceHeader{}))
					}

				})
			}
		}
	}
}

func createSqliteBlob(b *testing.B, name string, ctx *testcontext.Context, sync string) storage.Blobs {
	blobs, err := sqlite.NewBlobStore(name+".db", sync != "nosync")
	require.NoError(b, err)
	return blobs
}
func createFileBlob(b *testing.B, name string, ctx *testcontext.Context, sync string) storage.Blobs {
	workingDir := ctx.Dir("pieces")
	_ = os.MkdirAll(workingDir, 0755)
	dir, err := filestore.NewDir(zap.NewNop(), workingDir)
	require.NoError(b, err)
	if sync == "nosync" {
		dir.SkipSync = true
	}
	blobs := filestore.New(zap.NewNop(), dir, filestore.DefaultConfig)
	return blobs
}
