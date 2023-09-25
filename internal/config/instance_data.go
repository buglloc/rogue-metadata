package config

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/buglloc/rogue-metadata/internal/idp"
	"github.com/buglloc/rogue-metadata/internal/idp/fsprovider"
	"github.com/buglloc/rogue-metadata/internal/idp/proxyprovider"
	"github.com/buglloc/rogue-metadata/internal/idpsrv"
)

type InstanceDataProviderKind string

const (
	InstanceDataProviderKindNone  InstanceDataProviderKind = ""
	InstanceDataProviderKindFS    InstanceDataProviderKind = "fs"
	InstanceDataProviderKindProxy InstanceDataProviderKind = "proxy"
)

func (k *InstanceDataProviderKind) UnmarshalText(data []byte) error {
	switch strings.ToLower(string(data)) {
	case "", "none":
		*k = InstanceDataProviderKindNone
	case "fs":
		*k = InstanceDataProviderKindFS
	case "proxy":
		*k = InstanceDataProviderKindProxy
	default:
		return fmt.Errorf("invalid upstream kind: %s", string(data))
	}
	return nil
}

func (k InstanceDataProviderKind) MarshalText() ([]byte, error) {
	return []byte(k), nil
}

type InstanceDataProviderFS struct {
	Dir string `koanf:"dir"`
}

type InstanceDataProviderProxy struct {
	Upstream string `koanf:"upstream"`
}

type InstanceData struct {
	Listen   string                    `koanf:"listen"`
	Provider InstanceDataProviderKind  `koanf:"provider"`
	FS       InstanceDataProviderFS    `koanf:"fs"`
	Proxy    InstanceDataProviderProxy `koanf:"proxy"`
}

func (r *Runtime) NewDataProvider() (idp.Provider, error) {
	switch r.cfg.InstanceData.Provider {
	case InstanceDataProviderKindFS:
		return fsprovider.NewProvider(r.cfg.InstanceData.FS.Dir), nil
	case InstanceDataProviderKindProxy:
		return proxyprovider.NewProvider(r.cfg.InstanceData.Proxy.Upstream), nil
	default:
		return nil, fmt.Errorf("unsupported instance data provider: %s", r.cfg.InstanceData.Provider)
	}
}

func (r *Runtime) NewInstanceDataServer() (*http.Server, error) {
	dataProvider, err := r.NewDataProvider()
	if err != nil {
		return nil, fmt.Errorf("create data provider: %w", err)
	}

	return idpsrv.NewServer(r.cfg.InstanceData.Listen, dataProvider)
}
