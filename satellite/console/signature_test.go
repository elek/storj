package console

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSignatureVerification(t *testing.T) {
	pk, err := crypto.GenerateKey()
	require.NoError(t, err)

	t.Run("Sign message", func(t *testing.T) {
		signature, err := CreateSignature("test@storj.io", pk)
		require.NoError(t, err)

		err = CheckSignature("test@storj.io", crypto.CompressPubkey(&pk.PublicKey), signature)
		require.NoError(t, err)
	})

	t.Run("Sign with wrong email", func(t *testing.T) {
		signature, err := CreateSignature("asd@storj.io", pk)
		require.NoError(t, err)

		err = CheckSignature("test@storj.io", crypto.CompressPubkey(&pk.PublicKey), signature)
		require.Error(t, err)
	})


}
