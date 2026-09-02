package importer

import (
	"strings"
	"testing"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

const termiusSample = "\ufeff" + `Groups,Label,Tags,Hostname/IP,Protocol,Port,Username,Password
work,CashCow,"prod,billing",cashcow.example.com,ssh,22,root,s3cret
work/db,db primary,prod;db,10.1.0.5,ssh,2222,postgres,
,,,192.168.1.20,ssh,,pi,
home,router,,192.168.1.1,telnet,23,admin,pw
work,CashCow,,cashcow2.example.com,ssh,22,root,
work,,,,,,,
work,Weird * name?,,10.0.0.9,SSH,22,,
`

func TestParseTermius(t *testing.T) {
	hosts, warnings, err := ParseTermius(strings.NewReader(termiusSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 5 {
		t.Fatalf("got %d hosts: %+v", len(hosts), hosts)
	}
	cc := hosts[0]
	if cc.Host.Name != "CashCow" || cc.Host.Group != "work" || cc.Host.User != "root" || cc.Host.Port != 22 || cc.Password != "s3cret" || cc.Host.Auth != config.AuthPassword {
		t.Errorf("CashCow: %+v", cc)
	}
	if got := strings.Join(cc.Host.Tags, "|"); got != "prod|billing" {
		t.Errorf("tags = %q", got)
	}
	db := hosts[1]
	if db.Host.Name != "db-primary" || db.Host.Group != "work/db" || db.Host.Port != 2222 || db.Host.Auth != "" || db.Password != "" {
		t.Errorf("db: %+v", db)
	}
	if got := strings.Join(db.Host.Tags, "|"); got != "prod|db" {
		t.Errorf("db tags = %q", got)
	}
	pi := hosts[2]
	if pi.Host.Name != "192.168.1.20" || pi.Host.Group != "" || pi.Host.Port != 0 || pi.Host.User != "pi" {
		t.Errorf("unlabeled host: %+v", pi)
	}
	if hosts[3].Host.Name != "CashCow-2" || hosts[3].Host.Host != "cashcow2.example.com" {
		t.Errorf("duplicate label: %+v", hosts[3])
	}
	if hosts[4].Host.Name != "Weird-name" {
		t.Errorf("sanitized name: %+v", hosts[4])
	}
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"line 5: router uses telnet", "line 6: label \"CashCow\" renamed to CashCow-2", "line 7: no hostname", "renamed to Weird-name"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings lack %q:\n%s", want, joined)
		}
	}
	c := &config.Config{}
	plain := make([]config.Host, 0, len(hosts))
	for _, h := range hosts {
		plain = append(plain, h.Host)
	}
	res := Import(c, plain, false)
	if len(res.Added) != 5 {
		t.Errorf("added: %v", res.Added)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("imported config invalid: %v", err)
	}
	if c.Find("CashCow").Auth != config.AuthPassword || c.Find("db-primary").Auth != config.AuthAgent {
		t.Errorf("auth: %+v %+v", c.Find("CashCow"), c.Find("db-primary"))
	}
	if c.Find("192.168.1.20").Group != Group || c.GroupIndex("work/db") < 0 || c.GroupIndex(Group) < 0 {
		t.Errorf("groups: %+v", c.Groups)
	}
}

func TestParseTermiusBadInput(t *testing.T) {
	if _, _, err := ParseTermius(strings.NewReader("")); err == nil {
		t.Error("empty file must fail")
	}
	if _, _, err := ParseTermius(strings.NewReader("name,user\nfoo,bar\n")); err == nil || !strings.Contains(err.Error(), "Hostname/IP") {
		t.Errorf("unknown header: %v", err)
	}
	if _, _, err := ParseTermius(strings.NewReader("Label,Hostname/IP,Port\nx,1.2.3.4,99999\n")); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("bad port: %v", err)
	}
}
