package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	AuthKey      = "key"
	AuthAgent    = "agent"
	AuthPassword = "password"
)

var ErrNotFound = errors.New("hosts.yaml not found")

type Config struct {
	Version  int      `yaml:"version"`
	Defaults Defaults `yaml:"defaults"`
	Groups   []Group  `yaml:"groups,omitempty"`
	Hosts    []Host   `yaml:"hosts"`
}

type Defaults struct {
	User        string `yaml:"user,omitempty"`
	Port        int    `yaml:"port,omitempty"`
	Key         string `yaml:"key,omitempty"`
	SecretStore string `yaml:"secret_store,omitempty"`
}

type Group struct {
	Name  string `yaml:"name"`
	Color string `yaml:"color,omitempty"`
}

type Host struct {
	Name  string   `yaml:"name"`
	Host  string   `yaml:"host"`
	User  string   `yaml:"user,omitempty"`
	Port  int      `yaml:"port,omitempty"`
	Auth  string   `yaml:"auth,omitempty"`
	Key   string   `yaml:"key,omitempty"`
	Group string   `yaml:"group,omitempty"`
	Tags  []string `yaml:"tags,omitempty,flow"`
	Jump  string   `yaml:"jump,omitempty"`
	Note  string   `yaml:"note,omitempty"`
}

type Resolved struct {
	User string
	Port int
	Auth string
	Key  string
}

func Path() string {
	if p := os.Getenv("SV_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sv", "hosts.yaml")
}

func New() *Config {
	c := &Config{Version: 1, Defaults: Defaults{Port: 22}}
	if key := "~/.ssh/id_ed25519"; FileExists(ExpandHome(key)) {
		c.Defaults.Key = key
	}
	return c
}

func Load() (*Config, error) {
	return LoadFrom(Path())
}

func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) Save() error {
	return c.SaveTo(Path())
}

func (c *Config) SaveTo(path string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.Version == 0 {
		c.Version = 1
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return WriteAtomic(path, buf.Bytes(), 0o600)
}

func (c *Config) Validate() error {
	names := make(map[string]bool, len(c.Hosts))
	for i, h := range c.Hosts {
		pos := fmt.Sprintf("host #%d", i+1)
		if h.Name != "" {
			pos = fmt.Sprintf("host %q", h.Name)
		}
		if h.Name == "" {
			return fmt.Errorf("%s: name is empty", pos)
		}
		if strings.ContainsAny(h.Name, " \t*?!") {
			return fmt.Errorf("%s: name must not contain spaces or * ? !", pos)
		}
		if h.Host == "" {
			return fmt.Errorf("%s: host is empty", pos)
		}
		if h.Port < 0 || h.Port > 65535 {
			return fmt.Errorf("%s: port %d out of range", pos, h.Port)
		}
		switch h.Auth {
		case "", AuthKey, AuthAgent, AuthPassword:
		default:
			return fmt.Errorf("%s: auth must be key, agent or password, got %q", pos, h.Auth)
		}
		lower := strings.ToLower(h.Name)
		if names[lower] {
			return fmt.Errorf("%s: duplicate name", pos)
		}
		names[lower] = true
	}
	for _, h := range c.Hosts {
		if h.Jump == "" {
			continue
		}
		if strings.EqualFold(h.Jump, h.Name) {
			return fmt.Errorf("host %q: jump points to itself", h.Name)
		}
		if !names[strings.ToLower(h.Jump)] {
			return fmt.Errorf("host %q: jump %q is not a known host", h.Name, h.Jump)
		}
	}
	seen := make(map[string]bool, len(c.Groups))
	for i, g := range c.Groups {
		if g.Name == "" {
			return fmt.Errorf("group #%d: name is empty", i+1)
		}
		lower := strings.ToLower(g.Name)
		if seen[lower] {
			return fmt.Errorf("group %q: duplicate name", g.Name)
		}
		seen[lower] = true
	}
	switch c.Defaults.SecretStore {
	case "", "keychain", "file":
	default:
		return fmt.Errorf("defaults.secret_store must be keychain or file, got %q", c.Defaults.SecretStore)
	}
	return nil
}

func (c *Config) Resolve(h *Host) Resolved {
	r := Resolved{User: h.User, Port: h.Port, Auth: h.Auth, Key: h.Key}
	if r.User == "" {
		r.User = c.Defaults.User
	}
	if r.User == "" {
		r.User = currentUser()
	}
	if r.Port == 0 {
		r.Port = c.Defaults.Port
	}
	if r.Port == 0 {
		r.Port = 22
	}
	if r.Key == "" {
		r.Key = c.Defaults.Key
	}
	if r.Auth == "" {
		if h.Key != "" || (r.Key != "" && FileExists(ExpandHome(r.Key))) {
			r.Auth = AuthKey
		} else {
			r.Auth = AuthAgent
		}
	}
	if r.Auth != AuthKey {
		r.Key = ""
	}
	return r
}

func (c *Config) Find(name string) *Host {
	for i := range c.Hosts {
		if strings.EqualFold(c.Hosts[i].Name, name) {
			return &c.Hosts[i]
		}
	}
	return nil
}

func (c *Config) Match(query string) []*Host {
	q := strings.ToLower(query)
	var sub, seq []*Host
	for i := range c.Hosts {
		name := strings.ToLower(c.Hosts[i].Name)
		switch {
		case strings.Contains(name, q):
			sub = append(sub, &c.Hosts[i])
		case isSubsequence(q, name):
			seq = append(seq, &c.Hosts[i])
		}
	}
	if len(sub) > 0 {
		return sub
	}
	return seq
}

func (c *Config) Remove(name string) bool {
	for i := range c.Hosts {
		if strings.EqualFold(c.Hosts[i].Name, name) {
			c.Hosts = append(c.Hosts[:i], c.Hosts[i+1:]...)
			return true
		}
	}
	return false
}

func (c *Config) EnsureGroup(name string) {
	for _, g := range c.Groups {
		if strings.EqualFold(g.Name, name) {
			return
		}
	}
	c.Groups = append(c.Groups, Group{Name: name})
}

func (c *Config) RemoveGroup(name string) int {
	idx := c.GroupIndex(name)
	if idx < 0 {
		return -1
	}
	c.Groups = append(c.Groups[:idx], c.Groups[idx+1:]...)
	moved := 0
	for i := range c.Hosts {
		if strings.EqualFold(c.Hosts[i].Group, name) {
			c.Hosts[i].Group = ""
			moved++
		}
	}
	return moved
}

func (c *Config) GroupIndex(name string) int {
	for i, g := range c.Groups {
		if strings.EqualFold(g.Name, name) {
			return i
		}
	}
	return -1
}

func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}

func CollapseHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

func FileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func isSubsequence(q, s string) bool {
	if q == "" {
		return true
	}
	i := 0
	for _, ch := range s {
		if rune(q[i]) == ch {
			i++
			if i == len(q) {
				return true
			}
		}
	}
	return false
}

func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}

func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { os.Remove(tmpPath) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func (c *Config) GroupRank(group string) int {
	if group == "" {
		return len(c.Groups) + 1
	}
	if i := c.GroupIndex(group); i >= 0 {
		return i
	}
	return len(c.Groups)
}

func SortHosts(c *Config, s *State, hosts []*Host) {
	sort.SliceStable(hosts, func(i, j int) bool {
		a, b := hosts[i], hosts[j]
		if ra, rb := c.GroupRank(a.Group), c.GroupRank(b.Group); ra != rb {
			return ra < rb
		}
		if ga, gb := strings.ToLower(a.Group), strings.ToLower(b.Group); ga != gb {
			return ga < gb
		}
		if s != nil {
			if la, lb := s.Last(a.Name), s.Last(b.Name); !la.Equal(lb) {
				return la.After(lb)
			}
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}
