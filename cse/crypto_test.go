package cse

import (
	"bytes"
	"crypto/aes"
	"fmt"
	"os"
	"testing"
)

const (
	testEnc = "/tmp/test.out"
	testOut = "/tmp/test.dat"
	testRef = "testdata/vector01.dat"
	rPadRef = "testdata/vector02.dat"
)

var (
	testKey []byte
)

// FailWithError is a utility for dumping errors and failing the test.
func FailWithError(t *testing.T, err error) {
	fmt.Println("failed")
	if err != nil {
		fmt.Println("[!] ", err.Error())
	}
	t.FailNow()
}

// Test padding a single block.
func TestPadBlock(t *testing.T) {
	m := []byte("Hello, world.")
	p, err := PadBuffer(m)
	if len(p) != aes.BlockSize {
		FailWithError(t, err)
	}
	fmt.Println("ok")
}

// Test padding a longer block of data.
func TestPadBlock2(t *testing.T) {
	m := []byte("ABCDABCDABCDABCD")
	p, err := PadBuffer(m)
	if len(p) != (2 * aes.BlockSize) {
		FailWithError(t, err)
	}
	fmt.Println("ok")
}

func TestUnpadBlock(t *testing.T) {
	m := [][]byte{
		[]byte("ABCDABCDABCDABC"),
		[]byte("ABCDABCDABCDABCD"),
		[]byte("This is a much longer test message. It should still work."),
		[]byte("Hello, world."),
		[]byte("Halló, heimur."),
		[]byte("こんにちは、世界。"),
		[]byte("خوش آمدید، جهان است."),
		[]byte("Здравствуй, мир."),
	}
	for i := 0; i < len(m); i++ {
		p, err := PadBuffer(m[i])
		if err != nil {
			FailWithError(t, err)
		} else if len(p)%aes.BlockSize != 0 {
			err = fmt.Errorf("len(p): %d", len(p))
			FailWithError(t, err)
		}

		unpad, err := UnpadBuffer(p)
		if err != nil {
			FailWithError(t, err)
		} else if len(unpad) != len(m[i]) {
			err = fmt.Errorf("len(p): %d", len(p))
			FailWithError(t, err)
		} else if !bytes.Equal(unpad, m[i]) {
			err = fmt.Errorf("unpad == '%s'", string(unpad))
			FailWithError(t, err)
		}
	}
	fmt.Println("ok")
}

func TestRandom(t *testing.T) {
	n := []int{2, 8, 15, 32, 57}
	for _, i := range n {
		r, err := Random(i)
		if err != nil {
			FailWithError(t, err)
		}
		if len(r) != i {
			err = fmt.Errorf("Len %v doesn't match %v", r, i)
			FailWithError(t, err)
		}
	}
}

// Test session key generation.
func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil || len(key) != aes.BlockSize {
		FailWithError(t, err)
	}
}

// Test long term key generation.
func TestGenerateLTKey(t *testing.T) {
	if SecureLevel < 1 {
		err := fmt.Errorf("crypto library operating in degraded mode")
		FailWithError(t, err)
	}

	key, err := GenerateLTKey()
	if err != nil || len(key) != KeySize {
		FailWithError(t, err)
	}
}

// Test initialisation vector generation.
func TestGenerateIV(t *testing.T) {
	iv, err := GenerateIV()
	if err != nil || len(iv) != aes.BlockSize {
		FailWithError(t, err)
	}
}

// Test Zeroise, which is used in the EncryptReader
func TestZeroise(t *testing.T) {
	var err error
	var testVector = []byte("hello, world")

	if len(testVector) != len("hello, world") {
		err = fmt.Errorf("testVector improperly initialised")
		FailWithError(t, err)
	}

	Zeroise(&testVector)
	if len(testVector) != 0 {
		err = fmt.Errorf("testVector not empty after Zeroise")
		FailWithError(t, err)
	}
}

// Test the encryption of a file.
func TestEncryptReader(t *testing.T) {
	fmt.Printf("EncryptReader: ")
	const testFile = "testdata/vector01.dat"
	const testOut = "/tmp/test.out"
	var err error

	testKey, err = GenerateKey()
	if err != nil {
		FailWithError(t, err)
	}
	src, err := os.Open(testFile)
	if err != nil {
		FailWithError(t, err)
	}

	out, err := os.OpenFile(testOut, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		FailWithError(t, err)
	}

	err = EncryptReader(testKey, src, out)
	if err != nil {
		FailWithError(t, err)
	}
	out.Close()

	fi, err := os.Stat(testFile)
	if err != nil {
		FailWithError(t, err)
	}
	expected := (fi.Size()/BlockSize)*BlockSize + (2 * BlockSize)
	fi, err = os.Stat(testOut)
	if err != nil {
		err = fmt.Errorf("[testOut] %s", err.Error())
		FailWithError(t, err)
	}

	if expected != fi.Size() {
		err = fmt.Errorf("output file is the wrong size (%d instead of %d)",
			fi.Size(), expected)
	}
	if err != nil {
		FailWithError(t, err)
	}

	fmt.Println("ok")
}

// Test the encryption of a file.
func TestDecryptReader(t *testing.T) {
	fmt.Printf("DecryptReader: ")

	src, err := os.Open(testEnc)
	if err != nil {
		FailWithError(t, err)
	}

	out, err := os.OpenFile(testOut, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		FailWithError(t, err)
	}

	err = DecryptReader(testKey, src, out)
	if err != nil {
		FailWithError(t, err)
	}
	out.Close()

	fi, err := os.Stat(testRef)
	if err != nil {
		FailWithError(t, err)
	}
	expected := fi.Size()
	fi, err = os.Stat(testOut)
	if err != nil {
		err = fmt.Errorf("[testOut] %s", err.Error())
		FailWithError(t, err)
	}

	if expected != fi.Size() {
		err = fmt.Errorf("output file is the wrong size (%d instead of %d)",
			fi.Size(), expected)
	}

	os.Remove(testEnc)
	os.Remove(testOut)
	if err != nil {
		FailWithError(t, err)
	}

	fmt.Println("ok")
}

// func TestFail(t *testing.T) {
// 	err := fmt.Errorf("Errorrororo")
// 	FailWithError(t, err)
// 	return
// }

// Benchmark the generation of session keys.
func BenchmarkGenerateKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		key, err := GenerateKey()
		if err != nil || len(key) != KeySize {
			b.FailNow()
		}
		Zeroise(&key)
	}
}

// Benchmark the generation of long-term encryption keys.
func BenchmarkGenerateLTKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		key, err := GenerateLTKey()
		if err != nil || len(key) != KeySize {
			b.FailNow()
		}
		Zeroise(&key)
	}
}

// Benchmark the generation of initialisation vectors.
func BenchmarkGenerateIV(b *testing.B) {
	for i := 0; i < b.N; i++ {
		iv, err := GenerateIV()
		if err != nil || len(iv) != BlockSize {
			b.FailNow()
		}
		Zeroise(&iv)
	}
}

// Benchmark the scrubbing of a key.
func BenchmarkScrubKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		key, err := GenerateKey()
		if err != nil {
			b.FailNow()
		}

		err = Scrub(key, 3)
		if err != nil {
			b.FailNow()
		}
	}
}
