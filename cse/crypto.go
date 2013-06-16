// package cse provides client side encryption for wp/fs2s3.go
// This package borrows heavily from "Practical Cryptography With Go" by Kyle Isom
// https://leanpub.com/gocrypto
package cse

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

var (
	devRandom   *os.File
	SecureLevel = 0
)

func init() {
	var err error

	devRandom, err = os.Open("/dev/random")
	if err != nil {
		fmt.Fprintf(os.Stderr, "*** failed to open /dev/random")
		fmt.Fprintf(os.Stderr, "    long-term key generation not recommended")
	} else {
		SecureLevel = 1
	}
}

// pad the end of the message for block cipher
func PadBuffer(msg []byte) (pad []byte, err error) {
	msglen := len(msg)

	padding := aes.BlockSize - msglen%aes.BlockSize
	pad = make([]byte, msglen, msglen+padding)
	copy(pad, msg)
	pad = append(pad, 0x80)
	for i := 1; i < padding; i++ {
		pad = append(pad, 0x00)
	}
	return
}

// unpad padded buffer padded with PadBuffer
func UnpadBuffer(pad []byte) (msg []byte, err error) {
	msg = pad
	var padlen int
	origlen := len(msg)

	for padlen = origlen - 1; padlen >= 0; padlen-- {
		if msg[padlen] == 0x80 {
			break
		}
		if msg[padlen] != 0x00 || (origlen-padlen) > aes.BlockSize {
			err = PaddingError
			return
		}
	}
	msg = msg[:padlen]
	return
}

// Return size random bits
func Random(size int) (b []byte, err error) {
	b = make([]byte, size)
	_, err = io.ReadFull(rand.Reader, b)
	return
}

// Generates a symemetric key for aes block Cipher
func GenerateKey() (key []byte, err error) {
	return Random(aes.BlockSize)
}

// Generates a cryptographically strong symemetric key for aes block Cipher
func GenerateLTKey() (key []byte, err error) {
	if devRandom == nil {
		err = DegradedError
		return
	}
	key = make([]byte, KeySize)

	_, err = io.ReadFull(devRandom, key)
	return
}

// GenerateIV returns an initialisation vector suitable for
// AES-CBC encryption. I don't know why this is separate from
// the generateKey func...totally culted.
func GenerateIV() (iv []byte, err error) {
	return Random(aes.BlockSize)
}

// Zeroise wipes out the data in a slice before deleting the array.
func Zeroise(data *[]byte) (n int) {
	dLen := len(*data)

	for n = 0; n < dLen; n++ {
		(*data)[n] = 0x0
	}

	*data = make([]byte, 0)
	return
}

// Scrub writes random data to the variable the given number of
// rounds, then zeroises it.
func Scrub(data []byte, rounds int) (err error) {
	dLen := len(data)

	var n int
	for r := 0; r < rounds; r++ {
		for i := 0; i < dLen; i++ {
			n, err = io.ReadFull(rand.Reader, data)
			if err != nil {
				return
			} else if n != dLen {
				err = fmt.Errorf("[scrub] invalid random read size %d", n)
				return
			}
		}
	}
	if dLen != Zeroise(&data) {
		err = fmt.Errorf("zeroise failed")
	}
	return
}

// Encrypt an io.Reader to and io.Writer
func EncryptReader(key []byte, r io.Reader, w io.Writer) (err error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	iv, err := GenerateIV()
	if err != nil {
		return
	}

	n, err := w.Write(iv)
	if err != nil {
		return
	} else if n != BlockSize {
		err = IVSizeMismatchError
		return
	}

	cbc := cipher.NewCBCEncrypter(c, iv)

	cryptBlock := make([]byte, 0)

	for {
		if len(cryptBlock) == BlockSize {
			cbc.CryptBlocks(cryptBlock, cryptBlock)
			n, err = w.Write(cryptBlock)
			if err != nil {
				return
			} else if n != BlockSize {
				err = BlockSizeMismatchError
				return
			}
			Zeroise(&cryptBlock)
		}

		readLen := BlockSize - len(cryptBlock)
		buf := make([]byte, readLen)
		n, err = r.Read(buf)
		if err != nil && err != io.EOF {
			return
		} else if n > 0 {
			cryptBlock = append(cryptBlock, buf[0:n]...)
		}

		if err != nil && err == io.EOF {
			err = nil
			break
		}
	}

	cryptBlock, err = PadBuffer(cryptBlock)
	if err != nil {
		return
	} else if (len(cryptBlock) % BlockSize) != 0 {
		err = BlockSizeMismatchError
		return
	}
	cbc.CryptBlocks(cryptBlock, cryptBlock)
	n, err = w.Write(cryptBlock)
	if err != nil {
		return
	} else if n != BlockSize {
		err = BlockSizeMismatchError
	}
	return
}

// Decrypt an io.Reader to an io.Writer.
func DecryptReader(key []byte, r io.Reader, w io.Writer) (err error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	iv := make([]byte, BlockSize)
	n, err := r.Read(iv)
	if err != nil {
		return
	} else if n != BlockSize {
		err = IVSizeMismatchError
		return
	}

	cbc := cipher.NewCBCDecrypter(c, iv)

	// We use a cryptoBlock to differentiate between partial reads
	// and EOF conditions.
	cryptBlock := make([]byte, 0)

	for {
		if len(cryptBlock) == BlockSize {
			cbc.CryptBlocks(cryptBlock, cryptBlock)
			cryptBlock, err = unpadBlock(cryptBlock)
			if err != nil {
				return
			}
			n, err = w.Write(cryptBlock)
			if err != nil {
				return
			}
			Zeroise(&cryptBlock)
		}

		readLen := BlockSize - len(cryptBlock)
		buf := make([]byte, readLen)
		n, err = r.Read(buf)
		if err != nil && err != io.EOF {
			return
		} else if n > 0 {
			cryptBlock = append(cryptBlock, buf[0:n]...)
		}

		if err != nil && err == io.EOF {
			err = nil
			break
		}
	}

	if len(cryptBlock) > 0 {
		cryptBlock, err = UnpadBuffer(cryptBlock)
		if err != nil {
			return
		}

		cbc.CryptBlocks(cryptBlock, cryptBlock)
		n, err = w.Write(cryptBlock)
		if err != nil {
			return
		} else if n != BlockSize {
			err = BlockSizeMismatchError
		}
	}
	return
}

// What is this for?
func unpadBlock(p []byte) (m []byte, err error) {
	m = p
	origLen := len(m)

	if m[origLen-1] != 0x0 && m[origLen-1] != 0x80 {
		return
	}
	return UnpadBuffer(m)
}
