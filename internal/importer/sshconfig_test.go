package importer

import (
	"strings"
	"testing"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

const sample = `
# comment
Host github.com
    HostName github.com
    User git
    IdentityFile ~/.ssh/id_ed25519
    IdentityFile ~/.ssh/second
    IdentitiesOnly yes

Host crown-prod
    HostName 10.1.1.1
    User codex
    Port=2222
    IdentityFile "~/.ssh/codex key"
    ServerAliveInterval 30

Host *.internal
    User nobody

Host inner
    HostName 10.0.0.5
    ProxyJump crown-prod

Host lonely

Match host foo
    User matched

Host after-match
    HostName after.example.com

# >>> sv managed - не редактировать, изменения перезапишутся >>>
Host managed-one
    HostName 9.9.9.9
# <<< sv managed <<<

Host tail
    hostname tail.example.com
`

func TestParseSSHConfig(t *testing.T) {
	hosts, err := ParseSSHConfig(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]config.Host{}
	for _, h := range hosts {
		byName[h.Name] = h
	}
	if len(hosts) != 6 {
		t.Errorf("got %d hosts: %v", len(hosts), keys(byName))
	}
	if _, ok := byName["managed-one"]; ok {
		t.Error("managed block must be skipped")
	}
	if _, ok := byName["*.internal"]; ok {
		t.Error("wildcard host must be skipped")
	}
	gh := byName["github.com"]
	if gh.Host != "github.com" || gh.User != "git" || gh.Key != "~/.ssh/id_ed25519" {
		t.Errorf("github: %+v", gh)
	}
	cp := byName["crown-prod"]
	if cp.Host != "10.1.1.1" || cp.User != "codex" || cp.Port != 2222 || cp.Key != "~/.ssh/codex key" {
		t.Errorf("crown-prod: %+v", cp)
	}
	if byName["inner"].Jump != "crown-prod" {
		t.Errorf("inner: %+v", byName["inner"])
	}
	if l := byName["lonely"]; l.Host != "lonely" {
		t.Errorf("host without HostName should use alias: %+v", l)
	}
	if byName["after-match"].User != "" {
		t.Errorf("match block leaked: %+v", byName["after-match"])
	}
	if byName["tail"].Host != "tail.example.com" {
		t.Errorf("lowercase keyword: %+v", byName["tail"])
	}
}

func TestImport(t *testing.T) {
	hosts, _ := ParseSSHConfig(strings.NewReader(sample))
	c := &config.Config{Hosts: []config.Host{{Name: "Crown-Prod", Host: "old", Auth: config.AuthAgent}}}

	res := Import(c, hosts, false)
	if len(res.Skipped) != 1 || res.Skipped[0] != "crown-prod" {
		t.Errorf("skipped: %v", res.Skipped)
	}
	if len(res.Added) != 5 {
		t.Errorf("added: %v", res.Added)
	}
	if c.Find("Crown-Prod").Host != "old" {
		t.Error("existing host must be untouched without --overwrite")
	}
	if c.GroupIndex(Group) < 0 {
		t.Error("imported group not created")
	}
	for _, name := range res.Added {
		h := c.Find(name)
		if h.Group != Group {
			t.Errorf("%s group = %q", name, h.Group)
		}
		wantAuth := config.AuthAgent
		if h.Key != "" {
			wantAuth = config.AuthKey
		}
		if h.Auth != wantAuth {
			t.Errorf("%s auth = %q, key %q", name, h.Auth, h.Key)
		}
	}
	if c.Find("inner").Jump != "crown-prod" {
		t.Errorf("jump to existing host must be kept, got %q", c.Find("inner").Jump)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("imported config invalid: %v", err)
	}

	res = Import(c, hosts, true)
	if c.Find("crown-prod").Host != "10.1.1.1" || len(res.Skipped) != 0 {
		t.Errorf("overwrite failed: %+v, skipped %v", c.Find("crown-prod"), res.Skipped)
	}
}

func TestImportDropsUnknownJump(t *testing.T) {
	c := &config.Config{}
	res := Import(c, []config.Host{{Name: "a", Host: "h", Jump: "user@1.2.3.4:22"}}, false)
	if len(res.Warnings) != 1 || c.Find("a").Jump != "" {
		t.Errorf("warnings %v, jump %q", res.Warnings, c.Find("a").Jump)
	}
	if err := c.Validate(); err != nil {
		t.Error(err)
	}
}

func keys(m map[string]config.Host) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
