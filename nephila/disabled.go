package nephila

import (
	"fmt"

	"github.com/Azure/azure-container-networking/netlink"
)

// DisabledNephilaProvider is returned when the provider is set to "Disabled"
type DisabledNephilaProvider struct{}

func (fnp DisabledNephilaProvider) GetType() string {
	return Disabled
}

func (dnp DisabledNephilaProvider) AddNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error {
	return nil
}
func (dnp DisabledNephilaProvider) DeleteNetworkContainerRules(ovs NephilaOVSEndpoint, ncConfig interface{}) error {
	return nil
}
func (dnp DisabledNephilaProvider) ConfigureNetworkContainerLink(link *netlink.VEthLink, ncConfig NephilaNetworkContainerConfig) error {
	fmt.Printf("HERE DISABLED\n")
	return nil
}
func (dnp DisabledNephilaProvider) ConfigureNode(nodeConf NephilaNodeConfig, dncConf NephilaDNCConfig) (NephilaNodeConfig, error) {
	var nodeConfig NephilaNodeConfig
	return nodeConfig, nil
}
