package config

import "github.com/buglloc/rogue-metadata/internal/blackhole"

type DNS struct {
	Listen   string   `koanf:"listen"`
	Upstream string   `koanf:"upstream"`
	IDP      string   `koanf:"idp"`
	Names    []string `koanf:"names"`
	IPs      []string `koanf:"ips"`
}

func (r *Runtime) NewDNSServer() (*blackhole.Server, error) {
	return blackhole.NewServer(blackhole.Config(r.cfg.DNS))
}
