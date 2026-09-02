package importer

import (
	"fmt"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

const Group = "imported"

type Result struct {
	Added    []string
	Skipped  []string
	Warnings []string
}

func Import(c *config.Config, hosts []config.Host, overwrite bool) Result {
	var res Result
	for _, h := range hosts {
		if h.Group == "" {
			h.Group = Group
		}
		if h.Auth == "" {
			if h.Key != "" {
				h.Auth = config.AuthKey
			} else {
				h.Auth = config.AuthAgent
			}
		}
		if existing := c.Find(h.Name); existing != nil {
			if !overwrite {
				res.Skipped = append(res.Skipped, h.Name)
				continue
			}
			*existing = h
		} else {
			c.Hosts = append(c.Hosts, h)
		}
		c.EnsureGroup(h.Group)
		res.Added = append(res.Added, h.Name)
	}
	for i := range c.Hosts {
		h := &c.Hosts[i]
		if h.Jump == "" || c.Find(h.Jump) != nil {
			continue
		}
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s: ProxyJump %q is not a known host, dropped", h.Name, h.Jump))
		h.Jump = ""
	}
	return res
}
