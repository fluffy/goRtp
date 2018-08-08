package rtp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

const (
	Ke  byte = 0x00
	Ka  byte = 0x01
	Ks  byte = 0x02
	KCe byte = 0x03
	KCa byte = 0x04
	KCs byte = 0x05
)

type KDF struct {
	masterSalt []byte
	block      cipher.Block
}

func NewKDF(masterKey, masterSalt []byte) (*KDF, error) {
	if len(masterSalt) < 14 {
		zero := bytes.Repeat([]byte{0x00}, 14-len(masterSalt))
		masterSalt = append(masterSalt, zero...)
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}

	return &KDF{
		masterSalt: masterSalt,
		block:      block,
	}, nil
}

func (kdf KDF) Derive(label byte, index uint64, size int) []byte {
	indexVal := make([]byte, 6)
	for i := range indexVal {
		indexVal[5-i] = byte(index)
		index >>= 8
	}

	keyID := append([]byte{label}, indexVal...)

	x := make([]byte, len(kdf.masterSalt))
	copy(x, kdf.masterSalt)
	start := len(kdf.masterSalt) - len(keyID)
	for i := range keyID {
		x[start+i] ^= keyID[i]
	}

	zero := bytes.Repeat([]byte{0x00}, kdf.block.BlockSize()-len(x))
	iv := append(x, zero...)

	stream := cipher.NewCTR(kdf.block, iv)

	out := make([]byte, size)
	for i := range out {
		out[i] = 0x00
	}

	stream.XORKeyStream(out, out)
	return out
}

func (kdf KDF) getKeySize(cipher CipherID) (int, int, error) {
	var keySize, saltSize int
	switch cipher {
	case SRTP_AEAD_AES_128_GCM:
		keySize = 16
		saltSize = 12
	case SRTP_AEAD_AES_256_GCM:
		keySize = 32
		saltSize = 12
	default:
		return 0, 0, fmt.Errorf("Unsupported cipher: %04x", cipher)
	}

	return keySize, saltSize, nil
}

func (kdf KDF) DeriveForStream(cipher CipherID) ([]byte, []byte, error) {
	keySize, saltSize, err := kdf.getKeySize(cipher)
	if err != nil {
		return nil, nil, err
	}

	// TODO replace them with actual values
	roc := 0
	seq := 0

	key := kdf.Derive(Ke, (uint64(roc) << 16) + uint64(seq), keySize)
	salt := kdf.Derive(Ks, (uint64(roc) << 16) + uint64(seq), saltSize)
	return key, salt, nil
}

func (kdf KDF) DeriveForSRTCPStream(cipher CipherID, srtcpIndex uint32) ([]byte, []byte, error) {
	keySize, saltSize, err := kdf.getKeySize(cipher)
	if err != nil {
		return nil, nil, err
	}

	rtcpKey := kdf.Derive(KCe, uint64(srtcpIndex), keySize)
	rtcpSalt := kdf.Derive(KCs, uint64(srtcpIndex), saltSize)

	return rtcpKey, rtcpSalt, nil
}
