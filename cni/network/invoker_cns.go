package network

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/cnsclient"
	"github.com/Azure/azure-container-networking/network"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
	cniTypesCurr "github.com/containernetworking/cni/pkg/types/current"
)

const (
	cnsPort = 10090
)

type CNSIPAMInvoker struct {
	podName              string
	podNamespace         string
	primaryInterfaceName string
	cnsClient            *cnsclient.CNSClient
}

func NewCNSInvoker(podName, namespace string) (*CNSIPAMInvoker, error) {
	cnsURL := "http://localhost:" + strconv.Itoa(cnsPort)
	cnsclient.InitCnsClient(cnsURL)
	cnsClient, err := cnsclient.GetCnsClient()

	return &CNSIPAMInvoker{
		podName:      podName,
		podNamespace: namespace,
		cnsClient:    cnsClient,
	}, err
}

//Add uses the requestipconfig API in cns, and returns ipv4 and a nil ipv6 as CNS doesn't support IPv6 yet
func (invoker *CNSIPAMInvoker) Add(args *cniSkel.CmdArgs, nwCfg *cni.NetworkConfig, subnetPrefix *net.IPNet, options map[string]interface{}) (*cniTypesCurr.Result, *cniTypesCurr.Result, error) {
	var (
		result   *cniTypesCurr.Result
		resultV6 *cniTypesCurr.Result
	)

	// Parse Pod arguments.
	podInfo := cns.KubernetesPodInfo{PodName: invoker.podName, PodNamespace: invoker.podNamespace}
	orchestratorContext, err := json.Marshal(podInfo)

	response, err := invoker.cnsClient.RequestIPAddress(orchestratorContext)
	if err != nil {
		return nil, nil, err
	}

	podIPAddress := response.PodIpInfo.PodIPConfig.IPAddress
	ncSubnetPrefix := response.PodIpInfo.NetworkContainerPrimaryIPConfig.IPSubnet.PrefixLength
	ncPrimaryIP := response.PodIpInfo.NetworkContainerPrimaryIPConfig.IPSubnet.IPAddress
	gwIPAddress := response.PodIpInfo.NetworkContainerPrimaryIPConfig.GatewayIPAddress
	hostSubnet := response.PodIpInfo.HostPrimaryIPInfo.Subnet
	hostPrimaryIP := response.PodIpInfo.HostPrimaryIPInfo.PrimaryIP

	gw := net.ParseIP(gwIPAddress)
	if gw == nil {
		return nil, nil, fmt.Errorf("Gateway address %v from response is invalid", gw)
	}

	hostIP := net.ParseIP(hostPrimaryIP)
	if hostIP == nil {
		return nil, nil, fmt.Errorf("Host IP address %v from response is invalid", hostIP)
	}

	// set result ipconfig from CNS Response Body
	ip, ipnet, err := net.ParseCIDR(podIPAddress + "/" + fmt.Sprint(ncSubnetPrefix))
	if ip == nil {
		return nil, nil, fmt.Errorf("Unable to parse IP from response: %v", podIPAddress)
	}

	// get the name of the primary IP address
	_, hostIPNet, err := net.ParseCIDR(hostSubnet)
	if err != nil {
		return nil, nil, err
	}

	// set subnet prefix for host vm
	*subnetPrefix = *hostIPNet

	// construct ipnet for result
	resultIPnet := net.IPNet{
		IP:   ip,
		Mask: ipnet.Mask,
	}

	// set the NC Primary IP in options
	options[network.SNATIPKey] = ncPrimaryIP

	result = &cniTypesCurr.Result{
		IPs: []*cniTypesCurr.IPConfig{
			{
				Version: "4",
				Address: resultIPnet,
				Gateway: gw,
			},
		},
		Routes: []*cniTypes.Route{
			{
				Dst: network.Ipv4DefaultRouteDstPrefix,
				GW:  gw,
			},
		},
	}

	return result, resultV6, nil
}

// Delete calls into the releaseipconfiguration API in CNS
func (invoker *CNSIPAMInvoker) Delete(address net.IPNet, nwCfg *cni.NetworkConfig, options map[string]interface{}) error {

	// Parse Pod arguments.
	podInfo := cns.KubernetesPodInfo{PodName: invoker.podName, PodNamespace: invoker.podNamespace}

	orchestratorContext, err := json.Marshal(podInfo)
	if err != nil {
		return err
	}

	return invoker.cnsClient.ReleaseIPAddress(orchestratorContext)
}
