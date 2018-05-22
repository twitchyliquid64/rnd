package netctrl

import (
	"netctrl/hostapd"
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
		Subnet   string `json:"subnet"`
		Wireless struct {
			SSID string `json:"SSID"`
		} `json:"wireless"`
	} `json:"config"`

	AP *hostapd.APStatus `json:"AP"`
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
	out.Config.Wireless.SSID = c.config.Network.Wireless.SSID
	out.AP = c.lastAPState
	return out
}
