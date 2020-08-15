package network

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
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
	cnsPort       = 10090
	azureQueryUrl = "http://168.63.129.16/machine/plugins?comp=nmagent&type=getinterfaceinfov1"
)

var (
	ipv4DefaultRouteDstPrefix = net.IPNet{net.IPv4zero, net.IPv4Mask(0, 0, 0, 0)}
)

type nmAgentInterfacesResponse struct {
	XMLName   xml.Name           `xml:"Interfaces"`
	Interface []nmAgentInterface `xml:"Interface"`
}

type nmAgentInterface struct {
	MacAddress string            `xml:"MacAddress,attr"`
	IsPrimary  bool              `xml:"IsPrimary,attr"`
	IPSubnet   []nmAgentIPSubnet `xml:"IPSubnet"`
}

type nmAgentIPSubnet struct {
	Prefix    string             `xml:"Prefix,attr"`
	IPAddress []nmAgentIPAddress `xml:"IPAddress"`
}

type nmAgentIPAddress struct {
	Address   string `xml:"Address,attr"`
	IsPrimary bool   `xml:"IsPrimary,attr"`
}

type CNSIPAMInvoker struct {
	podName              string
	podNamespace         string
	primaryInterfaceName string
	cnsClient            *cnsclient.CNSClient
}

func getHostSubnet(queryUrl string) (*net.IPNet, error) {
	var (
		nmagent nmAgentInterfacesResponse
	)

	resp, err := http.DefaultClient.Get(azureQueryUrl)
	if err != nil {
		return nil, err
	}

	err = xml.NewDecoder(resp.Body).Decode(&nmagent)
	if err != nil {
		return nil, err
	}

	for _, vmInterface := range nmagent.Interface {
		if vmInterface.IsPrimary {
			if len(vmInterface.IPSubnet) == 0 {
				return nil, fmt.Errorf("No subnet found for primary interface in host response")
			}

			_, subnet, err := net.ParseCIDR(vmInterface.IPSubnet[0].Prefix)
			if err != nil {
				return nil, err
			}

			return subnet, nil
		}
	}

	return nil, fmt.Errorf("No primary host interface found from host response %v", nmagent)
}

func getIPv4AddressWithHostSubnet(hostSubnet *net.IPNet) (string, net.IP, error) {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {

		addrs, err := iface.Addrs()
		if err != nil {
			return "", nil, err
		}

		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if hostSubnet.Contains(ipnet.IP) {
					return iface.Name, ipnet.IP, nil
				}
			}
		}
	}

	return "", nil, fmt.Errorf("No interface on VM containing IP in supplied host subnet [%v] ", hostSubnet)
}

func SetSNATForPrimaryIP() {
	//cmd := "iptables -t nat -A POSTROUTING -s 10.0.0.0/24 -j SNAT –to-source 10.10.10.99"

	// Create separate chain from POSTROUTING
	// if destination ip is private, return from chain
	// if destination is public, snat
	//
	// Check vnet peering case
}

func NewCNSInvoker(podName, namespace string) (*CNSIPAMInvoker, error) {
	primaryMacAddress, err := getHostSubnet(azureQueryUrl)
	if err != nil {
		return nil, err
	}

	interfaceName, interfaceIP, err := getIPv4AddressWithHostSubnet(primaryMacAddress)
	if err != nil {
		return nil, err
	}

	cnsURL := "http://" + interfaceIP.String() + ":" + strconv.Itoa(cnsPort)
	cnsclient.InitCnsClient(cnsURL)
	cnsClient, err := cnsclient.GetCnsClient()

	return &CNSIPAMInvoker{
		podName:              podName,
		podNamespace:         namespace,
		primaryInterfaceName: interfaceName,
		cnsClient:            cnsClient,
	}, err
}

func (invoker *CNSIPAMInvoker) Add(args *cniSkel.CmdArgs, nwCfg *cni.NetworkConfig, nwInfo network.NetworkInfo, isDeletePoolOnError bool) (*cniTypesCurr.Result, *cniTypesCurr.Result, error) {
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

	response.IPConfiguration.IPSubnet.PrefixLength = 24

	// set result ipconfig from CNS Response Body
	ip, ipnet, err := response.IPConfiguration.IPSubnet.GetIPNet()
	if err != nil {
		return nil, nil, err
	}

	gw := net.ParseIP(response.IPConfiguration.GatewayIPAddress)
	if gw == nil {
		return nil, nil, fmt.Errorf("Gateway address %v from response is invalid", gw)
	}
	log.Printf("Using Gateway %v", gw)

	nwCfg.Master = invoker.primaryInterfaceName
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
		Routes: []*cniTypes.Route{
			{
				Dst: ipv4DefaultRouteDstPrefix,
				GW:  gw,
			},
		},
	}
	return result, resultV6, nil
}

func (invoker *CNSIPAMInvoker) Delete(address net.IPNet, nwCfg *cni.NetworkConfig, nwInfo network.NetworkInfo, isDeletePoolOnError bool) error {

	// Parse Pod arguments.
	podInfo := cns.KubernetesPodInfo{PodName: invoker.podName, PodNamespace: invoker.podNamespace}

	orchestratorContext, err := json.Marshal(podInfo)
	if err != nil {
		return err
	}

	nwInfo.MasterIfName = invoker.primaryInterfaceName
	return invoker.cnsClient.ReleaseIPAddress(orchestratorContext)
}
