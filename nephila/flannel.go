package nephila

import (
	"net"

	"github.com/Azure/azure-container-networking/netlink"
	"github.com/Azure/azure-container-networking/ovsctl"
)

const (
	Flannel  = "Flannel"
	Disabled = "Disabled"
)

// IPSubnet contains ip subnet.
type IPSubnet struct {
	IPAddress    string
	PrefixLength uint8
}

type FlannelDNCConfig struct {
	OverlaySubnet       string //IPSubnet // 169.254.0.0
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

type FlannelNephilaProvider struct {
}

func (fnp FlannelNephilaProvider) AddNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error {
	fncc := ncConfig.(NephilaNetworkContainerConfig)
	config := fncc.Config.(FlannelNetworkContainerConfig)
	nodeConfig := fncc.NodeConfig.(FlannelNodeConfig)

	overlayAddressSpace := nodeConfig.OverlaySubnet.IPAddress + "/" + string(nodeConfig.OverlaySubnet.PrefixLength)

	containerPort, err := ovsctl.GetOVSPortNumber(ovs.HostVethName)
	if err != nil {
		return err
	}

	ovsctl.AddOverlayIPDnatRule(ovs.BridgeName, config.OverlayIP, ovs.ContainerIP, ovs.VlanID, containerPort)
	ovsctl.AddOverlayIPSnatRule(ovs.BridgeName, containerPort, overlayAddressSpace, config.OverlayIP)
	ovsctl.AddOverlayFakeArpReply(ovs.BridgeName, config.OverlayIP, ovs.ContainerMac)
	return nil
}

func (fnp FlannelNephilaProvider) DeleteNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error {
	fncc := ncConfig.(NephilaNetworkContainerConfig)
	config := fncc.Config.(FlannelNetworkContainerConfig)
	nodeConfig := fncc.NodeConfig.(FlannelNodeConfig)

	overlayAddressSpace := nodeConfig.OverlaySubnet.IPAddress + "/" + string(nodeConfig.OverlaySubnet.PrefixLength)

	containerPort, err := ovsctl.GetOVSPortNumber(ovs.HostVethName)
	hostPort, err := ovsctl.GetOVSPortNumber(ovs.HostPrimaryIfName)
	if err != nil {
		return err
	}

	ovsctl.DeleteOverlayIPDnatRule(ovs.BridgeName, hostPort, config.OverlayIP)
	ovsctl.DeleteOverlayIPSnatRule(ovs.BridgeName, containerPort, overlayAddressSpace)
	ovsctl.DeleteOverlayFakeArpReply(ovs.BridgeName, config.OverlayIP)

	return nil
}

func (fnp FlannelNephilaProvider) ConfigureNode(dncConfig interface{}) (NephilaNodeConfig, error) {
	var nodeConfig NephilaNodeConfig

	err := StartFlannel(dncConfig.(FlannelDNCConfig))
	if err != nil {

	}
	flannelConf, err := GetFlannelConfiguration() // get the env's set by flannel
	nodeConfig.Config = flannelConf

	return nodeConfig, err
}

func (fnp FlannelNephilaProvider) ConfigureDNC(config interface{}) error {
	return nil
}

func (fnp FlannelNephilaProvider) ConfigureNetworkContainerLink(*netlink.VEthLink) {

}
