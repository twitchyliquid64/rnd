# Research Net Daemon

Run a wireless network from a Raspberry pi, which is always behind a VPN.

## Setup

#### Requirements

 * Raspberry Pi 3 (preferable 3 B+)
 * Go version 1.10+
 * A set of openVPN configurations

#### Install hostapd

```shell

sudo apt install -y openvpn easy-rsa hostapd
sudo systemctl mask hostapd.service
sudo systemctl mask wpa_supplicant.service
```

#### Install rnd

```shell

git clone https://github.com/twitchyliquid64/rnd
cd rnd
export GOPATH=`pwd`
go build -o rnd *.go #assumes you have Go install successfully.
```

#### Start daemon

Clear out any state on the wireless NIC: `ip addr flush dev wlan0 && ip link set dev wlan0 down`

Run it with your configuration file: `./rnd myconfig.hcl`

## Example config

```hcl

name = "VPN Controller"
listener = ":1234"

network = {
  interface_ident = "vpn"
  subnet = "192.168.101.1/24"
  wireless = {
    interface = "wlan0"
    SSID = "my_network_name"
    password = "my_password"
  }
}

vpn_configs = [
  {
    name = "USA config 1"
    icon = "flag-icon flag-icon-us"
    path = "us1.ovpn"
    username = "..."
    password = "..."
  },
  {
    name = "USA Config 2"
    icon = "flag-icon flag-icon-us"
    path = "us2.ovpn"
    username = "..."
    password = "..."
  }
]

firewall = {
  vpnbox_blocked_ports = [22, 80]
  blocked_subnets = [
    "192.168.1.1/24"
  ]
}
```
