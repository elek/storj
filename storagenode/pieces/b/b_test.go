package b

import (
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"os"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode/pieces"
	"testing"
)

func BenchmarkReadWrite(b *testing.B) {
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	// setup test parameters
	const blockSize = int(256 * memory.KiB)
	satelliteID := testrand.NodeID()
	source := testrand.Bytes(2319872)

	for _, hashAlgo := range []pb.PieceHashAlgorithm{pb.PieceHashAlgorithm_SHA256, pb.PieceHashAlgorithm_BLAKE3} {
		for _, sync := range []string{"sync", "nosync"} {
			b.Run(sync+"-"+hashAlgo.String(), func(b *testing.B) {
				workingDir := ctx.Dir("pieces")
				_ = os.MkdirAll(workingDir, 0755)
				dir, err := filestore.NewDir(zap.NewNop(), workingDir)
				require.NoError(b, err)
				if sync == "nosync" {
					dir.SkipSync = true
				}
				blobs := filestore.New(zap.NewNop(), dir, filestore.DefaultConfig)
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
