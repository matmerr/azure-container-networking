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
	GetType() string
	AddNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error
	DeleteNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error
	ConfigureNetworkContainerLink(link *netlink.VEthLink, ncConfig NephilaNetworkContainerConfig) error
	ConfigureNode(nodeConf NephilaNodeConfig, dncConf NephilaDNCConfig) (NephilaNodeConfig, error)
}

// NephilaOVSEndpoint is used exclusively for writing the required OVS rules
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
	Config interface{}
}

// NephilaNodeConfig contains the nephila type, and the NephilaConfig
type NephilaNodeConfig struct {
	Type   string
	Config interface{}
}

type NephilaNetworkContainerConfig struct {
	Type       string
	Config     interface{}
	NodeConfig interface{}
}
