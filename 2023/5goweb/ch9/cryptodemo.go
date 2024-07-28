package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

var commonIV = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

func main() {
	orgstring := "this is string"
	debyte := base64.StdEncoding.EncodeToString([]byte(orgstring))
	fmt.Println("debyte:", debyte)
	enbytebase, _ := base64.StdEncoding.DecodeString(debyte)
	enbyte := string(enbytebase)
	fmt.Println("enbyte:", enbyte)

	key_text := "12345678uytrewqplkjhgfds" //len must be 16/24/32

	c, err := aes.NewCipher([]byte(key_text))
	if err != nil {
		fmt.Println("error:", err)
	}

	plaintext := orgstring // []byte(orgstring)
	//加密
	cfb := cipher.NewCFBEncrypter(c, commonIV)
	ciphertext := make([]byte, len(plaintext))
	cfb.XORKeyStream(ciphertext, []byte(plaintext))
	fmt.Printf("%s => %x\n", plaintext, ciphertext)

	// 解密
	cfbdec := cipher.NewCFBDecrypter(c, commonIV)
	plaintextCopy := make([]byte, len(plaintext))
	cfbdec.XORKeyStream(plaintextCopy, ciphertext)
	fmt.Printf("%x => %s\n", ciphertext, plaintextCopy)
}
