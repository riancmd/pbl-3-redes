package blockchain

import (
	"PlanoZ/internal/models"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
)

// verifica se a assinatura da transacao bate com a chave publica enviada
func VerifySignature(publicKeyBytes []byte, UserData []string, signature []byte) bool {
	jsonData, err := json.Marshal(UserData)

	if err != nil {
		slog.Error("error while encoding json")
		return false
	}

	// tenta reconstruir as chaves a partir dos bytes
	curve := elliptic.P256()
	x, y := elliptic.Unmarshal(curve, publicKeyBytes)
	if x == nil || y == nil {
		slog.Error("VerifySignature: falha ao decodificar chave pública",
			"pubKeyLen", len(publicKeyBytes),
			"firstByte", fmt.Sprintf("%02x", publicKeyBytes[0]))
		return false
	}

	// monta a chave publica struct
	publicKey := ecdsa.PublicKey{Curve: curve, X: x, Y: y}

	// separa o R e o S da assinatura
	r := big.Int{}
	s := big.Int{}
	signatureLen := len(signature)
	r.SetBytes(signature[:(signatureLen / 2)])
	s.SetBytes(signature[(signatureLen / 2):])

	// bate o hash dos dados com a assinatura e a chave
	hash := sha256.Sum256(jsonData)
	return ecdsa.Verify(&publicKey, hash[:], &r, &s)
}

// valida o request que vem direto do client (antes de virar tx)
func VerifyTransactionRequestSignature(req models.TransactionRequest) bool {
	// IMPORTANTE: a ordem aqui tem que ser EXATAMENTE a mesma que o cliente usou pra assinar
	// senao o hash sai diferente e a verificacao falha
	dataToVerify := []string{
		req.Payload,
		fmt.Sprintf("%d", req.Timestamp), // timestamp como string
		req.UserID,
		string(req.Type),
	}

	// log pra debug se der erro de assinatura
	slog.Info("Verificando assinatura",
		"payload", req.Payload,
		"timestamp", req.Timestamp,
		"userID", req.UserID,
		"type", req.Type)

	// serializa
	jsonData, err := json.Marshal(dataToVerify)
	if err != nil {
		slog.Error("VerifyRequest: error encoding json", "error", err)
		return false
	}

	// gera hash
	hash := sha256.Sum256(jsonData)

	// chama a validacao interna
	valid := verifyECDSA(req.PublicKey, req.Signature, hash[:])

	if !valid {
		slog.Error("Assinatura inválida detectada",
			"userID", req.UserID,
			"dataHash", fmt.Sprintf("%x", hash[:8]),
			"pubKeyLen", len(req.PublicKey),
			"sigLen", len(req.Signature))
	}

	return valid
}

// funcao auxiliar pra fazer a checagem das curvas elipticas
func verifyECDSA(pubKeyBytes []byte, sigBytes []byte, hash []byte) bool {
	// 1. checagem basica de tamanho
	if len(pubKeyBytes) == 0 || len(sigBytes) == 0 {
		slog.Error("verifyECDSA: dados vazios",
			"pubKeyLen", len(pubKeyBytes),
			"sigLen", len(sigBytes))
		return false
	}

	curve := elliptic.P256()

	// 2. reconstroi chave publica (o marshal do cliente add um prefixo 0x04)
	x, y := elliptic.Unmarshal(curve, pubKeyBytes)
	if x == nil || y == nil {
		slog.Error("verifyECDSA: falha ao decodificar chave pública",
			"pubKeyLen", len(pubKeyBytes),
			"firstByte", fmt.Sprintf("%02x", pubKeyBytes[0]))
		return false
	}

	publicKey := ecdsa.PublicKey{Curve: curve, X: x, Y: y}

	// 3. reconstroi assinatura (r, s)
	sigLen := len(sigBytes)
	if sigLen < 2 {
		slog.Error("verifyECDSA: assinatura muito curta", "len", sigLen)
		return false
	}

	r := new(big.Int).SetBytes(sigBytes[:(sigLen / 2)])
	s := new(big.Int).SetBytes(sigBytes[(sigLen / 2):])

	// 4. verifica de fato
	valid := ecdsa.Verify(&publicKey, hash, r, s)

	if !valid {
		slog.Error("verifyECDSA: ecdsa.Verify retornou false",
			"rLen", len(sigBytes[:(sigLen/2)]),
			"sLen", len(sigBytes[(sigLen/2):]))
	}

	return valid
}
