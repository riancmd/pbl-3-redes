package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"math/big"
)

// verifica se a assinatura é correta
func VerifySignature(publicKeyBytes []byte, data []byte, signature []byte) bool {
	// reconstruindo a assinatura nas diferentes variáveis
	curve := elliptic.P256()
	x := big.Int{}
	y := big.Int{}
	keyLen := len(publicKeyBytes)
	x.SetBytes(publicKeyBytes[:(keyLen / 2)])
	y.SetBytes(publicKeyBytes[(keyLen / 2):])

	// agora, coloca a chave pública
	publicKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}

	// separando a assinatura em r e s
	r := big.Int{}
	s := big.Int{}
	signatureLen := len(signature)
	r.SetBytes(signature[:(signatureLen / 2)])
	s.SetBytes(signature[(signatureLen / 2):])

	// faz a verificação com a chave pública, o hash e a assinatura
	hash := sha256.Sum256(data)
	return ecdsa.Verify(&publicKey, hash[:], &r, &s)
}
