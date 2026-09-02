package importer

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

func ParseSSHConfig(r io.Reader) ([]config.Host, error) {
	var hosts []config.Host
	var cur []config.Host
	flush := func() {
		hosts = append(hosts, cur...)
		cur = nil
	}
	inManaged := false
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "# >>> sv managed") {
			flush()
			inManaged = true
			continue
		}
		if strings.HasPrefix(line, "# <<< sv managed") {
			inManaged = false
			continue
		}
		if inManaged || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val := splitKV(line)
		switch strings.ToLower(key) {
		case "host":
			flush()
			for _, alias := range strings.Fields(val) {
				if strings.ContainsAny(alias, "*?!") {
					continue
				}
				cur = append(cur, config.Host{Name: alias, Host: alias})
			}
		case "match":
			flush()
		case "hostname":
			for i := range cur {
				cur[i].Host = val
			}
		case "user":
			for i := range cur {
				cur[i].User = val
			}
		case "port":
			p, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("bad port %q", val)
			}
			for i := range cur {
				cur[i].Port = p
			}
		case "identityfile":
			for i := range cur {
				if cur[i].Key == "" {
					cur[i].Key = val
				}
			}
		case "proxyjump":
			for i := range cur {
				cur[i].Jump = val
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flush()
	return hosts, nil
}

func splitKV(line string) (string, string) {
	i := strings.IndexAny(line, " \t=")
	if i == -1 {
		return line, ""
	}
	key := line[:i]
	val := strings.TrimLeft(line[i:], " \t=")
	val = strings.TrimSpace(val)
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		val = val[1 : len(val)-1]
	}
	return key, val
}
