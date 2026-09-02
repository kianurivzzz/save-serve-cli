package keychain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

const (
	service = "sv"

	StoreKeychain = "keychain"
	StoreFile     = "file"
)

var ErrNotFound = errors.New("no password stored")

type Store interface {
	Kind() string
	Get(name string) (string, error)
	Set(name, password string) error
	Delete(name string) error
}

func Open(kind string) (Store, error) {
	file := &fileStore{path: FilePath()}
	switch kind {
	case StoreKeychain:
		return composite{primary: keyringStore{}, fallback: file}, nil
	case StoreFile:
		return composite{primary: file, fallback: keyringStore{}}, nil
	case "":
		if keyringAvailable() {
			return composite{primary: keyringStore{}, fallback: file}, nil
		}
		return composite{primary: file, fallback: nil}, nil
	}
	return nil, fmt.Errorf("unknown secret store %q, want %s or %s", kind, StoreKeychain, StoreFile)
}

type composite struct {
	primary  Store
	fallback Store
}

func (c composite) Kind() string { return c.primary.Kind() }

func (c composite) Get(name string) (string, error) {
	pw, err := c.primary.Get(name)
	if !errors.Is(err, ErrNotFound) || c.fallback == nil {
		return pw, err
	}
	return c.fallback.Get(name)
}

func (c composite) Set(name, password string) error {
	return c.primary.Set(name, password)
}

func (c composite) Delete(name string) error {
	err := c.primary.Delete(name)
	if c.fallback == nil {
		return err
	}
	ferr := c.fallback.Delete(name)
	if errors.Is(err, ErrNotFound) {
		return ferr
	}
	return err
}

func FilePath() string {
	return filepath.Join(filepath.Dir(config.Path()), "secrets.yaml")
}

func account(name string) string {
	return strings.ToLower(name)
}

func keyringAvailable() bool {
	_, err := keyring.Get(service, "__probe__")
	return err == nil || errors.Is(err, keyring.ErrNotFound)
}

type keyringStore struct{}

func (keyringStore) Kind() string { return StoreKeychain }

func (keyringStore) Get(name string) (string, error) {
	pw, err := keyring.Get(service, account(name))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return pw, err
}

func (keyringStore) Set(name, password string) error {
	return keyring.Set(service, account(name), password)
}

func (keyringStore) Delete(name string) error {
	err := keyring.Delete(service, account(name))
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

type fileStore struct {
	path string
}

func (*fileStore) Kind() string { return StoreFile }

func (s *fileStore) load() (map[string]string, error) {
	m := map[string]string{}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", s.path, err)
	}
	return m, nil
}

func (s *fileStore) save(m map[string]string) error {
	if len(m) == 0 {
		err := os.Remove(s.path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return config.WriteAtomic(s.path, data, 0o600)
}

func (s *fileStore) Get(name string) (string, error) {
	m, err := s.load()
	if err != nil {
		return "", err
	}
	pw, ok := m[account(name)]
	if !ok {
		return "", ErrNotFound
	}
	return pw, nil
}

func (s *fileStore) Set(name, password string) error {
	m, err := s.load()
	if err != nil {
		return err
	}
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "warning: keychain is unavailable, storing password in %s in plain text (protected by file mode 0600 only)\n", s.path)
	}
	m[account(name)] = password
	return s.save(m)
}

func (s *fileStore) Delete(name string) error {
	m, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := m[account(name)]; !ok {
		return ErrNotFound
	}
	delete(m, account(name))
	return s.save(m)
}
