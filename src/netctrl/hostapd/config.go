package hostapd

import (
	"bytes"
	"config"
	"html/template"
)

var baseConfigTmpl = template.Must(template.New("name").Parse(`
interface={{.WlanInterface}}
driver={{.HostapdDriver}}

ssid={{.SSID}}
hw_mode=g
channel=7
wmm_enabled=0
macaddr_acl=0
auth_algs=1
ignore_broadcast_ssid=0
wpa=2
wpa_passphrase={{.Passphrase}}
wpa_key_mgmt=WPA-PSK
wpa_pairwise=TKIP
rsn_pairwise=CCMP

bridge={{.BridgeName}}
ctrl_interface=/var/run/hostapd`))

// GenerateConfig creates hostapd configuration.
func GenerateConfig(c *config.Config) (string, error) {
	var b bytes.Buffer

	data := struct {
		SSID          string
		WlanInterface string
		Passphrase    string
		BridgeName    string
		HostapdDriver string
	}{
		SSID:          c.Network.Wireless.SSID,
		WlanInterface: c.Network.Wireless.Interface,
		Passphrase:    c.Network.Wireless.Password,
		BridgeName:    "br" + c.Network.InterfaceIdent,
		HostapdDriver: c.Network.Wireless.HostapdDriver,
	}

	if err := baseConfigTmpl.Execute(&b, &data); err != nil {
		return "", err
	}
	return b.String(), nil
}
