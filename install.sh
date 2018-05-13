#!/bin/bash

sudo apt install -y openvpn easy-rsa hostapd
sudo systemctl mask hostapd.service
sudo systemctl mask wpa_supplicant.service

