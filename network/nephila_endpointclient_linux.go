package network

type NephilaEndpointClient struct {
	bridgeName        string
	hostPrimaryIfName string
	hostVethName      string
	hostPrimaryMac    string
	containerVethName string
	containerMac      string
	snatVethName      string
}

func NewNephilaEndpointClient(
	extIf *externalInterface,
	epInfo *EndpointInfo,
	hostVethName string,
	containerVethName string,
) *NephilaEndpointClient {
	client := &NephilaEndpointClient{
		bridgeName:        extIf.BridgeName,
		hostPrimaryIfName: extIf.Name,
		hostVethName:      hostVethName,
		hostPrimaryMac:    extIf.MacAddress.String(),
		containerVethName: containerVethName,
	}
	return client
}

func (client *NephilaEndpointClient) AddEndpoints(epInfo *EndpointInfo) error {
	return nil
}
func (client *NephilaEndpointClient) AddEndpointRules(epInfo *EndpointInfo) error {
	return nil
}
func (client *NephilaEndpointClient) DeleteEndpointRules(ep *endpoint) {

}
func (client *NephilaEndpointClient) MoveEndpointsToContainerNS(epInfo *EndpointInfo, nsID uintptr) error {
	return nil
}
func (client *NephilaEndpointClient) SetupContainerInterfaces(epInfo *EndpointInfo) error {
	return nil
}
func (client *NephilaEndpointClient) ConfigureContainerInterfacesAndRoutes(epInfo *EndpointInfo) error {
	return nil
}
func (client *NephilaEndpointClient) DeleteEndpoints(ep *endpoint) error {
	return nil
}
