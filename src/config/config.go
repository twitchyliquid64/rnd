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
		Wireless       struct {
			Interface     string `hcl:"interface"`
			SSID          string `hcl:"SSID"`
			Password      string `hcl:"password"`
			HostapdDriver string `hcl:"hostapd_driver"`
		} `hcl:"wireless"`
	} `hcl:"network"`

	Debug struct {
		DHCP    bool `hcl:"dhcp"`
		Hostapd bool `hcl:"hostapd"`
	} `hcl:"debug"`

	VPNConfigurations []VPNOpt `hcl:"vpn_configs"`

	Firewall struct {
		VPNBoxBlockedPorts []int    `hcl:"vpnbox_blocked_ports"`
		BlockedSubnets     []string `hcl:"blocked_subnets"`
	} `hcl:"firewall"`
}

// VPNOpt represents one option for configuring the VPN.
type VPNOpt struct {
	Name string `hcl:"name" json:"name"`
	Path string `hcl:"path" json:"path"`
	Icon string `hcl:"icon" json:"icon"`

	Username string `hcl:"username" json:"-"`
	Password string `hcl:"password" json:"-"`
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
	if c.Network.Wireless.HostapdDriver == "" {
		c.Network.Wireless.HostapdDriver = "nl80211"
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
