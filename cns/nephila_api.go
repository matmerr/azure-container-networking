package cns

type NephilaConfig struct {
	Type   string
	Config FlannelDNCConfig // TODO: interface this and override marshall/unmarshall
}

type NephilaConfigResponse struct {
	Response   Response
	NodeConfig FlannelNodeConfig
}

type FlannelDNCConfig struct {
	OverlaySubnet       IPSubnet // 169.254.0.0
	PerNodePrefixLength uint8
}

type FlannelNodeConfig struct {
	NodeSubnet   IPSubnet
	InterfaceMTU int64
	IPMASQ       bool
}
