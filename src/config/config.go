package config

import (
	"errors"
	"io/ioutil"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
)

// Config stores configuration.
type Config struct {
	Name     string `hcl:"name"`
	Listener string `hcl:"listener"`

	Network struct {
		InterfaceIdent string `hcl:"interface_ident"`
		Subnet         string `hcl:"subnet"`
		WlanInterface  string `hcl:"wireless_interface"`
	} `hcl:"network"`

	VPNConfigurations []VPNOpt `hcl:"vpn_configs"`
}

// VPNOpt represents one option for configuring the VPN.
type VPNOpt struct {
	Name string `hcl:"name"`
	Path string `hcl:"path"`
	Icon string `hcl:"icon"`

	Username string `hcl:"username"`
	Password string `hcl:"password"`
}

func loadConfig(data []byte) (*Config, error) {
	astRoot, err := hcl.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	if _, ok := astRoot.Node.(*ast.ObjectList); !ok {
		return nil, errors.New("schema malformed")
	}

	var c Config
	err = hcl.DecodeObject(&c, astRoot)
	if err != nil {
		return nil, err
	}

	if err := validate(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

// LoadConfigFile loads configuration from the given file.
func LoadConfigFile(fpath string) (*Config, error) {
	d, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	return loadConfig(d)
}

func validate(c *Config) error {
	if c.Listener == "" {
		return errors.New("listener must be specified")
	}
	if c.Network.InterfaceIdent == "" {
		return errors.New("network.interface_ident must be specified")
	}
	if c.Network.Subnet == "" {
		return errors.New("network.subnet must be specified")
	}
	return nil
}
