package netctrl

import (
	"time"
)

// ControllerState represents the state of the network controller.
type ControllerState struct {
	Breaker struct {
		Tripped bool      `json:"tripped"`
		Updated time.Time `json:"last_updated"`
	} `json:"breaker"`

	// current configuration.
	Config struct {
		VPN struct {
			Configured bool   `json:"configured"`
			Name       string `json:"name"`
			Icon       string `json:"icon"`
		} `json:"vpn"`
		Subnet string `json:"subnet"`
	} `json:"config"`
}

// GetState returns the status of the controller.
func (c *Controller) GetState() *ControllerState {
	out := &ControllerState{}
	out.Breaker.Tripped = c.breakerTripped
	out.Breaker.Updated = c.breakerUpdated
	out.Config.Subnet = c.subnet.String()
	out.Config.VPN.Configured = c.vpnInterface != nil
	out.Config.VPN.Name = c.vpnConf.Name
	out.Config.VPN.Icon = c.vpnConf.Icon
	return out
}
