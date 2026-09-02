package importer

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

const termiusHeader = "Groups,Label,Tags,Hostname/IP,Protocol,Port,Username,Password"

type TermiusHost struct {
	Host     config.Host
	Password string
}

var termiusColumns = map[string]string{
	"groups":      "group",
	"group":       "group",
	"label":       "name",
	"name":        "name",
	"tags":        "tags",
	"tag":         "tags",
	"hostname/ip": "host",
	"hostname":    "host",
	"host":        "host",
	"address":     "host",
	"ip":          "host",
	"protocol":    "protocol",
	"port":        "port",
	"username":    "user",
	"user":        "user",
	"password":    "password",
}

func ParseTermius(r io.Reader) ([]TermiusHost, []string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil, errors.New("file is empty")
	}
	if err != nil {
		return nil, nil, err
	}
	cols := map[string]int{}
	for i, name := range header {
		name = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))
		field, ok := termiusColumns[name]
		if _, seen := cols[field]; ok && !seen {
			cols[field] = i
		}
	}
	if _, ok := cols["host"]; !ok {
		return nil, nil, fmt.Errorf("no Hostname/IP column, expected a Termius CSV export with header: %s", termiusHeader)
	}
	var hosts []TermiusHost
	var warnings []string
	taken := map[string]bool{}
	for line := 2; ; line++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		cell := func(field string) string {
			i, ok := cols[field]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		host := cell("host")
		if host == "" {
			if strings.TrimSpace(strings.Join(rec, "")) != "" {
				warnings = append(warnings, fmt.Sprintf("line %d: no hostname, skipped", line))
			}
			continue
		}
		label := cell("name")
		if label == "" {
			label = host
		}
		if p := strings.ToLower(cell("protocol")); p != "" && p != "ssh" {
			warnings = append(warnings, fmt.Sprintf("line %d: %s uses %s, only ssh is supported, skipped", line, label, p))
			continue
		}
		h := config.Host{
			Host:  host,
			User:  cell("user"),
			Group: cell("group"),
			Tags:  splitTags(cell("tags")),
		}
		if port := cell("port"); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return nil, nil, fmt.Errorf("line %d: bad port %q", line, port)
			}
			h.Port = n
		}
		h.Name = hostName(label, taken)
		if h.Name != label {
			warnings = append(warnings, fmt.Sprintf("line %d: label %q renamed to %s", line, label, h.Name))
		}
		pw := cell("password")
		if pw != "" {
			h.Auth = config.AuthPassword
		}
		hosts = append(hosts, TermiusHost{Host: h, Password: pw})
	}
	return hosts, warnings, nil
}

func hostName(label string, taken map[string]bool) string {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune("*?!", r) {
			return -1
		}
		return r
	}, label)
	base := strings.Join(strings.Fields(clean), "-")
	if base == "" {
		base = "host"
	}
	name := base
	for n := 2; taken[strings.ToLower(name)]; n++ {
		name = fmt.Sprintf("%s-%d", base, n)
	}
	taken[strings.ToLower(name)] = true
	return name
}

func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
