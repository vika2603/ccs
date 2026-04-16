package archive

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plain := []byte("super-secret-token")
	enc, err := EncryptPassphrase(plain, "correct horse battery staple")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	dec, err := DecryptPassphrase(enc, "correct horse battery staple")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(dec) != string(plain) {
		t.Errorf("got %q", dec)
	}
}

func TestDecryptWrongPassphraseFails(t *testing.T) {
	enc, _ := EncryptPassphrase([]byte("x"), "pass")
	if _, err := DecryptPassphrase(enc, "nope"); err == nil {
		t.Errorf("expected decrypt failure")
	}
}
