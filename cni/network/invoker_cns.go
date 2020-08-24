package network

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/cnsclient"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
	cniTypesCurr "github.com/containernetworking/cni/pkg/types/current"
)

const (
	cnsPort = 10090
)

var (
	ipv4DefaultRouteDstPrefix = net.IPNet{net.IPv4zero, net.IPv4Mask(0, 0, 0, 0)}
)

type CNSIPAMInvoker struct {
	podName              string
	podNamespace         string
	primaryInterfaceName string
	cnsClient            *cnsclient.CNSClient
}

func getHostInterfaceName(hostSubnet *net.IPNet, hostIP net.IP) (string, error) {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if hostSubnet.Contains(ipnet.IP) {
					if !ipnet.IP.Equal(hostIP) {
						return "", fmt.Errorf("Host IP specified by CNS and IMDS does not match IP found on host interface")
					}

					return iface.Name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("No interface on VM containing IP in supplied host subnet [%v] ", hostSubnet)
}

//TODO, once pod info is returned with Primary IP of NC
func SetSNATForPrimaryIP() {
	//cmd := "iptables -t nat -A POSTROUTING -s 10.0.0.0/24 -j SNAT –to-source 10.10.10.99"

	// Create separate chain from POSTROUTING
	// if destination ip is private, return from chain
	// if destination is public, snat
	// Check vnet peering case
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
func (invoker *CNSIPAMInvoker) Add(args *cniSkel.CmdArgs, nwCfg *cni.NetworkConfig, nwInfo network.NetworkInfo, options map[string]string) (*cniTypesCurr.Result, *cniTypesCurr.Result, error) {
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

	// set result ipconfig from CNS Response Body
	ip, ipnet, err := net.ParseCIDR(response.PodIpInfo.PodIPConfig.IPAddress + "/" + fmt.Sprint(response.PodIpInfo.NetworkContainerPrimaryIPConfig.IPSubnet.PrefixLength))
	if ip == nil {
		return nil, nil, fmt.Errorf("Unable to parse IP from response: %v", response.PodIpInfo.PodIPConfig.IPAddress)
	}

	gw := net.ParseIP(response.PodIpInfo.NetworkContainerPrimaryIPConfig.GatewayIPAddress)
	if gw == nil {
		return nil, nil, fmt.Errorf("Gateway address %v from response is invalid", gw)
	}

	// get the name of the primary IP address
	_, hostIPNet, err := net.ParseCIDR(response.PodIpInfo.HostPrimaryIPInfo.Subnet)
	if err != nil {
		return nil, nil, err
	}

	hostIP := net.ParseIP(response.PodIpInfo.HostPrimaryIPInfo.PrimaryIP)

	interfaceName, err := getHostInterfaceName(hostIPNet, hostIP)

	nwCfg.Master = interfaceName
	log.Printf("Setting master interface to %v", nwInfo.MasterIfName)

	// construct ipnet for result
	resultIPnet := net.IPNet{
		IP:   ip,
		Mask: ipnet.Mask,
	}

	result = &cniTypesCurr.Result{
		IPs: []*cniTypesCurr.IPConfig{
			{
				Version: "4",
				Address: resultIPnet,
				Gateway: gw,
			},
		},
		Routes: []*cniTypes.Route{},
	}
	return result, resultV6, nil
}

// Delete calls into the releaseipconfiguration API in CNS
func (invoker *CNSIPAMInvoker) Delete(address net.IPNet, nwCfg *cni.NetworkConfig, nwInfo network.NetworkInfo, options map[string]string) error {

	// Parse Pod arguments.
	podInfo := cns.KubernetesPodInfo{PodName: invoker.podName, PodNamespace: invoker.podNamespace}

	orchestratorContext, err := json.Marshal(podInfo)
	if err != nil {
		return err
	}

	nwInfo.MasterIfName = invoker.primaryInterfaceName
	return invoker.cnsClient.ReleaseIPAddress(orchestratorContext)
}
