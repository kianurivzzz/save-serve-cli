package keychain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestFileStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "secrets.yaml")
	s := &fileStore{path: path}
	if _, err := s.Get("a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty store: %v", err)
	}
	if err := s.Set("CashCow", "s3cret"); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o", st.Mode().Perm())
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "cashcow: s3cret") {
		t.Errorf("unexpected content:\n%s", raw)
	}
	if pw, err := s.Get("cashcow"); err != nil || pw != "s3cret" {
		t.Errorf("get: %q %v", pw, err)
	}
	if err := s.Delete("CASHCOW"); err != nil {
		t.Errorf("delete: %v", err)
	}
	if err := s.Delete("cashcow"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("empty secrets file should be removed")
	}
}

func TestKeyringStore(t *testing.T) {
	keyring.MockInit()
	s := keyringStore{}
	if _, err := s.Get("a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	if err := s.Set("Host", "pw"); err != nil {
		t.Fatal(err)
	}
	if pw, err := s.Get("HOST"); err != nil || pw != "pw" {
		t.Errorf("get: %q %v", pw, err)
	}
	if err := s.Delete("host"); err != nil {
		t.Errorf("delete: %v", err)
	}
	if err := s.Delete("host"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
}

func TestOpen(t *testing.T) {
	if s, err := Open(StoreFile); err != nil || s.Kind() != StoreFile {
		t.Errorf("file: %v %v", s, err)
	}
	if s, err := Open(StoreKeychain); err != nil || s.Kind() != StoreKeychain {
		t.Errorf("keychain: %v %v", s, err)
	}
	if _, err := Open("vault"); err == nil {
		t.Error("unknown store must fail")
	}
}

func TestCompositeFallback(t *testing.T) {
	keyring.MockInit()
	file := &fileStore{path: filepath.Join(t.TempDir(), "secrets.yaml")}
	c := composite{primary: keyringStore{}, fallback: file}
	if err := file.Set("a", "from-file"); err != nil {
		t.Fatal(err)
	}
	if pw, err := c.Get("a"); err != nil || pw != "from-file" {
		t.Errorf("fallback get: %q %v", pw, err)
	}
	if err := c.Set("a", "from-keyring"); err != nil {
		t.Fatal(err)
	}
	if pw, _ := c.Get("a"); pw != "from-keyring" {
		t.Errorf("primary must win: %q", pw)
	}
	if err := c.Delete("a"); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := c.Get("a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete must clear both stores: %v", err)
	}
	if err := c.Delete("a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete of missing: %v", err)
	}
}
