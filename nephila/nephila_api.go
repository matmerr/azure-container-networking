package nephila

import (
	"github.com/Azure/azure-container-networking/netlink"
)

const (
	Type       = "Type"
	Config     = "Config"
	NodeConfig = "NodeConfig"
)

//Keys in the NephilaNCMap

type NephilaProvider interface {
	AddNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error
	DeleteNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error
	ConfigureNetworkContainerLink(*netlink.VEthLink)
	ConfigureNode(dncConfig interface{}) (NephilaNodeConfig, error)
}

type NephilaOVSEndpoint struct {
	BridgeName        string
	HostPrimaryIfName string
	HostVethName      string
	ContainerMac      string
	ContainerIP       string
	VlanID            int
}

type NephilaDNCConfig struct {
	Type   string
	Config interface{} //FlannelDNCConfig // TODO: interface this and override marshall/unmarshall
}

type NephilaNodeConfig struct {
	Type   string
	Config interface{} //FlannelNodeConfig
}

type NephilaNetworkContainerConfig struct {
	Type       string
	Config     interface{} //FlannelNetworkContainerConfig `json:"Config"`
	NodeConfig interface{} //FlannelNodeConfig             `json:"NodeConfig"`
}
