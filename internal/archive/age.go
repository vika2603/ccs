package archive

import (
	"bytes"
	"fmt"
	"io"

	"filippo.io/age"
)

func EncryptPassphrase(plain []byte, passphrase string) ([]byte, error) {
	r, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, r)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plain); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecryptPassphrase(enc []byte, passphrase string) ([]byte, error) {
	id, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, err
	}
	r, err := age.Decrypt(bytes.NewReader(enc), id)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return io.ReadAll(r)
}
