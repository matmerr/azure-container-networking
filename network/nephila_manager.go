package network

import (
	"net"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/netlink"
	"github.com/Azure/azure-container-networking/ovsctl"
)

func NewNephilaNetworkContainerManager(nncc cns.NephilaNetworkContainerConfig) NephilaNetworkContainerManager {
	nphm := NephilaNetworkContainerManager{
		NephilaNCConfig: nncc,
	}
	return nphm
}

type NephilaNetworkContainerManager struct {
	// NephilaNCConfig could have the functions in this package, but
	// Configs not actionable, Managers are actionable
	NephilaNCConfig cns.NephilaNetworkContainerConfig
}

func (nephm *NephilaNetworkContainerManager) ConfigureNephilaLink(link *netlink.VEthLink) {
	if nephm.NephilaNCConfig.Type == cns.Flannel {
		link.MTU = uint(nephm.NephilaNCConfig.NodeConfig.InterfaceMTU)
	}
}

func (nephm *NephilaNetworkContainerManager) ConfigureNephilaEndpointRules(bridgeName string, containerActualIP net.IP, vlanid int, containerBridgePort string, containerMac string) {
	if nephm.NephilaNCConfig.Type == cns.Flannel {

		overlayIP := nephm.NephilaNCConfig.Config.OverlayIP
		overlayAddressSpace := nephm.NephilaNCConfig.NodeConfig.OverlaySubnet.IPAddress + "/" + string(nephm.NephilaNCConfig.NodeConfig.OverlaySubnet.PrefixLength)

		ovsctl.AddOverlayIPDnatRule(bridgeName, nephm.NephilaNCConfig.Config.OverlayIP, containerActualIP, vlanid, containerBridgePort)
		ovsctl.AddOverlayIPSnatRule(bridgeName, containerBridgePort, overlayAddressSpace, overlayIP)
		ovsctl.AddOverlayFakeArpReply(bridgeName, overlayIP, containerMac)
	}
}
