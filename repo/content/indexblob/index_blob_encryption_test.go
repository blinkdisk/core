package indexblob

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/blinkdisk/core/internal/blobcrypto"
	"github.com/blinkdisk/core/internal/blobtesting"
	"github.com/blinkdisk/core/internal/gather"
	"github.com/blinkdisk/core/internal/testlogging"
	"github.com/blinkdisk/core/repo/blob"
	"github.com/blinkdisk/core/repo/encryption"
	"github.com/blinkdisk/core/repo/format"
	"github.com/blinkdisk/core/repo/hashing"
	"github.com/blinkdisk/core/repo/logging"
)

type failingEncryptor struct {
	encryption.Encryptor
	err error
}

func (f failingEncryptor) Encrypt(input gather.Bytes, contentID []byte, output *gather.WriteBuffer) error {
	return f.err
}

func TestEncryptedBlobManager(t *testing.T) {
	data := blobtesting.DataMap{}
	st := blobtesting.NewMapStorage(data, nil, nil)
	fs := blobtesting.NewFaultyStorage(st)
	f := &format.ContentFormat{
		Hash:       hashing.DefaultAlgorithm,
		Encryption: encryption.DefaultAlgorithm,
	}
	hf, err := hashing.CreateHashFunc(f)
	require.NoError(t, err)
	enc, err := encryption.CreateEncryptor(f)
	require.NoError(t, err)

	ebm := EncryptionManager{
		st:             fs,
		crypter:        blobcrypto.StaticCrypter{Hash: hf, Encryption: enc},
		indexBlobCache: nil,
		log:            logging.NullLogger,
	}

	ctx := testlogging.Context(t)

	bm, err := ebm.EncryptAndWriteBlob(ctx, gather.FromSlice([]byte{1, 2, 3}), "x", "session1")
	require.NoError(t, err)

	stbm, err := st.GetMetadata(ctx, bm.BlobID)
	require.NoError(t, err)

	require.Equal(t, stbm, bm)

	var tmp gather.WriteBuffer
	defer tmp.Close()

	require.NoError(t, ebm.GetEncryptedBlob(ctx, bm.BlobID, &tmp))

	// data corruption
	data[bm.BlobID][0] ^= 1

	require.Error(t, ebm.GetEncryptedBlob(ctx, bm.BlobID, &tmp))

	require.ErrorIs(t, ebm.GetEncryptedBlob(ctx, "no-such-blob", &tmp), blob.ErrBlobNotFound)

	someError := errors.New("some error")

	fs.AddFault(blobtesting.MethodPutBlob).ErrorInstead(someError)

	_, err = ebm.EncryptAndWriteBlob(ctx, gather.FromSlice([]byte{1, 2, 3, 4}), "x", "session1")
	require.ErrorIs(t, err, someError)

	someError2 := errors.New("some error 2")

	ebm.crypter = blobcrypto.StaticCrypter{Hash: hf, Encryption: failingEncryptor{nil, someError2}}

	_, err = ebm.EncryptAndWriteBlob(ctx, gather.FromSlice([]byte{1, 2, 3, 4}), "x", "session1")
	require.ErrorIs(t, err, someError2)
}
