package console

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/zeebo/errs"
	"strconv"
)

const MessageTemplate = "Here I prove that my Storj account uses email %s on Satellite X"

func CheckSignature(email string, publicKey []byte, signature []byte) (err error) {
	hash := hashMessage([]byte(fmt.Sprintf(MessageTemplate, email)))
	signature = signature[:len(signature)-1] // remove recovery id

	checked := crypto.VerifySignature(publicKey, hash, signature)
	if !checked {
		//TODO fix this
		return errs.New("Signature is invalid")
	}
	return nil
}

func PublicKeyFromSignature(email string, sig []byte) (*ecdsa.PublicKey, error) {
	hash := hashMessage([]byte(fmt.Sprintf(MessageTemplate, email)))
	sig[64] -= 27
	return crypto.SigToPub(hash, sig)
}

func CreateSignature(email string, privateKey *ecdsa.PrivateKey) (signature []byte, err error) {
	hash := hashMessage([]byte(fmt.Sprintf(MessageTemplate, email)))

	signature, err = crypto.Sign(hash, privateKey)
	if err != nil {
		return nil, err
	}
	signature[len(signature)-1] = signature[len(signature)-1] + 27
	return signature, nil

}

func hashMessage(message []byte) []byte {
	prefix := []byte("\x19Ethereum Signed Message:\n" + strconv.Itoa(len(message)))
	var fullMessage = append(prefix, message...)
	hash := crypto.Keccak256Hash(fullMessage).Bytes()
	return hash
}
