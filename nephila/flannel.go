package nephila

import (
	"log"
	"net"
	"strconv"

	"github.com/Azure/azure-container-networking/netlink"
	"github.com/Azure/azure-container-networking/ovsctl"
)

const (
	// Flannel type
	Flannel = "Flannel"

	// Disabled type
	Disabled = "Disabled"
)

// IPSubnet contains ip subnet.
type IPSubnet struct {
	IPAddress    string
	PrefixLength uint8
}

type FlannelDNCConfig struct {
	OverlaySubnet       IPSubnet //IPSubnet // 169.254.0.0
	PerNodePrefixLength uint8
}

type FlannelNodeConfig struct {
	NodeSubnet    IPSubnet
	InterfaceMTU  int64
	IPMASQ        bool
	OverlaySubnet IPSubnet
}

// FlannelNetworkContainerConfig contains the overlay IP which has been assigned
type FlannelNetworkContainerConfig struct {
	OverlayIP net.IP
}

// FlannelNephilaProvider is just a struct to match the NephilaProvider interface
type FlannelNephilaProvider struct{}

func (fnp FlannelNephilaProvider) GetType() string {
	return Flannel
}

func (fnp FlannelNephilaProvider) AddNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error {
	log.Printf("HERE ADD RULES\n")
	fncc := ncConfig.(NephilaNetworkContainerConfig)
	config := fncc.Config.(FlannelNetworkContainerConfig)
	nodeConfig := fncc.NodeConfig.(FlannelNodeConfig)

	overlayAddressSpace := nodeConfig.OverlaySubnet.IPAddress + "/" + strconv.Itoa(int(nodeConfig.OverlaySubnet.PrefixLength))

	containerPort, err := ovsctl.GetOVSPortNumber(ovs.HostVethName)
	if err != nil {
		return err
	}
	log.Printf("HERE Overlay Address Space: %v\n", overlayAddressSpace)
	ovsctl.AddOverlayIPDnatRule(ovs.BridgeName, config.OverlayIP, ovs.ContainerIP, ovs.VlanID, containerPort)
	ovsctl.AddOverlayIPSnatRule(ovs.BridgeName, containerPort, overlayAddressSpace, config.OverlayIP)
	ovsctl.AddOverlayFakeArpReply(ovs.BridgeName, config.OverlayIP, ovs.ContainerMac)
	return nil
}

func (fnp FlannelNephilaProvider) DeleteNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error {
	/*
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
	*/
	return nil
}

func (fnp FlannelNephilaProvider) ConfigureNode(nodeConf NephilaNodeConfig, dncConf NephilaDNCConfig) (NephilaNodeConfig, error) {
	var nodeConfig NephilaNodeConfig

	// dependency on etcd, need mock to test
	err := SetFlannelKey(dncConf.Config.(FlannelDNCConfig))
	if err != nil {
		log.Printf("[Azure CNS Nephila: Flannel] Failed to set flannel etcd key with error %s\n", err.Error())
	}
	flannelConf, err := GetFlannelConfiguration() // get the env's set by flannel
	nodeConfig.Type = Flannel
	nodeConfig.Config = flannelConf

	return nodeConfig, err
}

func (fnp FlannelNephilaProvider) ConfigureNetworkContainerLink(link *netlink.VEthLink, ncConfig NephilaNetworkContainerConfig) error {
	fNodeConf := ncConfig.NodeConfig.(FlannelNodeConfig)
	link.LinkInfo.MTU = uint(fNodeConf.InterfaceMTU)
	return nil
}
