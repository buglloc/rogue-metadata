package config

import (
	"fmt"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Verbose      bool         `koanf:"verbose"`
	DNS          DNS          `koanf:"dns"`
	InstanceData InstanceData `koanf:"data"`
}

type Runtime struct {
	cfg *Config
}

func LoadConfig(files ...string) (*Config, error) {
	out := Config{
		Verbose: true,
		DNS: DNS{
			Listen:   ":53",
			Upstream: "1.1.1.1:53",
			Names: []string{
				"does-not-exist.example.com.",
				"example.invalid.",
			},
			IPs: []string{
				"169.254.169.254",
				"fd00:ec2::254",
			},
		},
		InstanceData: InstanceData{
			Listen:   ":8773",
			Provider: InstanceDataProviderKindProxy,
			Proxy: InstanceDataProviderProxy{
				Upstream: "169.254.169.254:80",
			},
			FS: InstanceDataProviderFS{
				Dir: "./data",
			},
		},
	}

	k := koanf.New(".")
	if err := k.Load(env.Provider("RCI", "_", nil), nil); err != nil {
		return nil, fmt.Errorf("load env config: %w", err)
	}

	yamlParser := yaml.Parser()
	for _, fpath := range files {
		if fpath == "" {
			continue
		}

		if err := k.Load(file.Provider(fpath), yamlParser); err != nil {
			return nil, fmt.Errorf("load %q config: %w", fpath, err)
		}
	}

	return &out, k.Unmarshal("", &out)
}

func (c *Config) NewRuntime() (*Runtime, error) {
	return &Runtime{
		cfg: c,
	}, nil
}
