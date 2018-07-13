package cns

import "net"

type NephilaDNCConfig struct {
	Type   string
	Config FlannelDNCConfig // TODO: interface this and override marshall/unmarshall
}

type NephilaNodeConfig struct {
	Type   string
	Config FlannelNodeConfig
}

type NephilaNetworkContainerConfig struct {
	Type       string
	Config     FlannelNetworkContainerConfig `json:"Config"`
	NodeConfig FlannelNodeConfig             `json:"NodeConfig"`
}

type NephilaConfigResponse struct {
	Response   Response
	NodeConfig NephilaNodeConfig
}

type FlannelDNCConfig struct {
	OverlaySubnet       IPSubnet // 169.254.0.0
	PerNodePrefixLength uint8
}

type FlannelNodeConfig struct {
	NodeSubnet    IPSubnet
	InterfaceMTU  int64
	IPMASQ        bool
	OverlaySubnet IPSubnet
}

type FlannelNetworkContainerConfig struct {
	OverlayIP net.IP
}
