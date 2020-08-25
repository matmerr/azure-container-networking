package network

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/cnsclient"
	ipt "github.com/Azure/azure-container-networking/iptables"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/netlink"
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

// SetSNATForPrimaryIP add's the snatting rules
// Example, ncSubnetAddressSpace = 10.0.0.0, ncSubnetPrefix = 24
func SetSNATForPrimaryIP(ncPrimaryIP string, ncSubnetAddressSpace string, ncSubnetPrefix uint8) (err error) {
	// Create SWIFT chain, this checks if the chain already exists
	err = ipt.CreateChain(ipt.V4, ipt.Nat, ipt.Swift)
	if err != nil {
		return
	}

	// add jump to SWIFT chain from POSTROUTING
	err = ipt.AppendIptableRule(ipt.V4, ipt.Nat, ipt.Postrouting, "", ipt.Swift)
	if err != nil {
		return
	}

	// don't snat private address space traffic
	privateAddressSpaceCondition := fmt.Sprint("-d 10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")
	err = ipt.InsertIptableRule(ipt.V4, ipt.Nat, ipt.Swift, privateAddressSpaceCondition, ipt.Return)
	if err != nil {
		return
	}

	// snat public IP address space
	snatPublicTrafficCondition := fmt.Sprintf("-m addrtype ! --dst-type local -s %s/%d", ncSubnetAddressSpace, ncSubnetPrefix)
	snatPrimaryIPJump := fmt.Sprintf("%s --to %s", ipt.Snat, ncPrimaryIP)
	err = ipt.AppendIptableRule(ipt.V4, ipt.Nat, ipt.Swift, snatPublicTrafficCondition, snatPrimaryIPJump)
	return
}

// SetNCAddressSpaceOnHostBrige Add's the NC subnet space to the primary interface
func SetNCAddressSpaceOnHostBrige(ncSubnetAddressSpace string, ncSubnetPrefix uint8, hostPrimaryIfName string) error {
	_, dst, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ncSubnetAddressSpace, ncSubnetPrefix))
	if err != nil {
		return fmt.Errorf("Failed to parse address space for adding to bridge: %v", err)
	}

	devIf, _ := net.InterfaceByName(hostPrimaryIfName)
	ifIndex := devIf.Index
	family := netlink.GetIpAddressFamily(ipv4DefaultRouteDstPrefix.IP)

	nlRoute := &netlink.Route{
		Family:    family,
		Dst:       dst,
		Gw:        ipv4DefaultRouteDstPrefix.IP,
		LinkIndex: ifIndex,
	}

	if err := netlink.AddIpRoute(nlRoute); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "file exists") {
			return fmt.Errorf("Failed to add route to host interface with error: %v", err)
		}
		log.Printf("[cni-cns-net] route already exists: dst %+v, gw %+v, interfaceName %v", nlRoute.Dst, nlRoute.Gw, hostPrimaryIfName)
	}
	return nil
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

	podIPAddress := response.PodIpInfo.PodIPConfig.IPAddress
	ncSubnet := response.PodIpInfo.NetworkContainerPrimaryIPConfig.IPSubnet.IPAddress
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

	// set host ip interface name
	hostInterfaceName, err := getHostInterfaceName(hostIPNet, hostIP)
	nwCfg.Master = hostInterfaceName

	// snat all internet traffic with NC primary IP, leave private traffic untouched
	err = SetSNATForPrimaryIP(ncPrimaryIP, ncSubnet, ncSubnetPrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to set snat rule for Primary NC IP: %v, NC Subnet %v/%v, with error: %v", ncPrimaryIP, ncSubnet, ncSubnetPrefix, err)
	}

	err = SetNCAddressSpaceOnHostBrige(ncSubnet, ncSubnetPrefix, hostInterfaceName)
	if err != nil {
		log.Printf("Failed add address space on host primary interface: %v IP: %v, NC Subnet %v/%v, with error: %v", hostInterfaceName, ncPrimaryIP, ncSubnet, ncSubnetPrefix, err)
		return nil, nil, fmt.Errorf("Failed add address space on host primary interface: %v IP: %v, NC Subnet %v/%v, with error: %v", hostInterfaceName, ncPrimaryIP, ncSubnet, ncSubnetPrefix, err)
	}

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
		Routes: []*cniTypes.Route{
			{
				Dst: ipv4DefaultRouteDstPrefix,
				GW:  gw,
			},
		},
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
