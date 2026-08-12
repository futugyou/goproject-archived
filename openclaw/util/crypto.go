package util

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// 用私钥对数据进行 RSA-SHA256 (PKCS1v15) 签名
func SignData(signingInput string, privateKeyPem string) ([]byte, error) {
	// 1. 解析 PEM 块
	block, _ := pem.Decode([]byte(privateKeyPem))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing private key")
	}

	// 2. 解析私钥 (兼容 PKCS#8 和 PKCS#1 格式)
	var privateKey *rsa.PrivateKey
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// 如果 PKCS#8 失败，尝试按照传统的 PKCS#1 格式解析
		pkcs1Key, errPkcs1 := x509.ParsePKCS1PrivateKey(block.Bytes)
		if errPkcs1 != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		privateKey = pkcs1Key
	} else {
		var ok bool
		privateKey, ok = parsedKey.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
	}

	// 3. 计算原文的 SHA256 哈希值
	hash := sha256.Sum256([]byte(signingInput))

	// 4. 使用 PKCS1v15 签名
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("error signing data: %v", err)
	}

	return signature, nil
}
