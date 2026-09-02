package connect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

func TestArgs(t *testing.T) {
	key := filepath.Join(t.TempDir(), "id")
	os.WriteFile(key, nil, 0o600)
	c := &config.Config{
		Defaults: config.Defaults{User: "root", Port: 22, Key: key},
		Hosts: []config.Host{
			{Name: "k", Host: "1.2.3.4", Auth: config.AuthKey},
			{Name: "a", Host: "2.2.2.2", User: "ops", Port: 2222, Auth: config.AuthAgent},
			{Name: "j", Host: "3.3.3.3", Auth: config.AuthAgent, Jump: "a"},
			{Name: "p", Host: "4.4.4.4", Auth: config.AuthPassword},
		},
	}
	cases := []struct {
		host   string
		remote []string
		want   string
	}{
		{"k", nil, "ssh -p 22 -i " + key + " -o IdentitiesOnly=yes -- root@1.2.3.4"},
		{"a", []string{"docker", "ps"}, "ssh -p 2222 -- ops@2.2.2.2 docker ps"},
		{"j", []string{"-weird"}, "ssh -p 22 -J a -- root@3.3.3.3 -weird"},
	}
	for _, tc := range cases {
		got, err := Args(c, c.Find(tc.host), tc.remote)
		if err != nil {
			t.Errorf("%s: %v", tc.host, err)
			continue
		}
		if s := strings.Join(got, " "); s != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.host, s, tc.want)
		}
	}
	if _, err := Args(c, c.Find("p"), nil); err == nil || !strings.Contains(err.Error(), "password") {
		t.Errorf("password host must be rejected in M1, got %v", err)
	}
}
