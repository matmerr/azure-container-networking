package ipam

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultWindowsKubePath           = `c:\k\`
	defaultWindowsKubeConfigFilePath = defaultWindowsKubePath + `config`
	defaultLinuxKubeConfigFilePath   = "/var/lib/kubelet/kubeconfig"
	nodeSubnetMask                   = "::/120"
)

type ipv6IpamSource struct {
	name           string
	kubeConfigPath string
	kubeClient     *kubernetes.Clientset
	kubeNode       *v1.Node
	sink           addressConfigSink
}

func newIPv6IpamSource(options map[string]interface{}) (*ipv6IpamSource, error) {
	var kubeConfigPath string
	name := options[common.OptEnvironment].(string)

	if runtime.GOOS == windows {
		kubeConfigPath = defaultWindowsKubeConfigFilePath
	} else {
		kubeConfigPath = defaultLinuxKubeConfigFilePath
	}

	return &ipv6IpamSource{
		name:           name,
		kubeConfigPath: kubeConfigPath,
	}, nil
}

// Starts the MAS source.
func (source *ipv6IpamSource) start(sink addressConfigSink) error {
	source.sink = sink

	return nil
}

// Stops the MAS source.
func (source *ipv6IpamSource) stop() {
	source.sink = nil
}

func (source *ipv6IpamSource) loadKubernetesConfig() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", source.kubeConfigPath)
	client, err := kubernetes.NewForConfig(config)
	return client, err
}

func (source *ipv6IpamSource) refresh() error {
	nodeName, err := os.Hostname()

	if source.kubeClient == nil || source.kubeNode == nil {
		source.kubeClient, err = source.loadKubernetesConfig()
		source.kubeNode, err = source.kubeClient.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	}

	if err != nil {
		return err
	}

	log.Printf("[ipam] Discovered CIDR's %v.", source.kubeNode.Spec.PodCIDRs)
	if err != nil {
		return err
	}

	// Query the list of Kubernetes Pod IPs
	interfaceIPs, err := retrieveKubernetesPodIPs(source.kubeNode, nodeSubnetMask)

	// Configure the local default address space.
	local, err := source.sink.newAddressSpace(LocalDefaultAddressSpaceId, LocalScope)
	if err != nil {
		return err
	}

	for _, i := range interfaceIPs.Interfaces {
		for el, s := range i.IPSubnets {
			_, subnet, err := net.ParseCIDR(s.Prefix)
			ifaceName := "azure-" + strconv.Itoa(el)
			priority := 0
			ap, err := local.newAddressPool(ifaceName, priority, subnet)

			for _, a := range s.IPAddresses {
				address := net.ParseIP(a.Address)

				_, err = ap.newAddressRecord(&address)
				if err != nil {
					log.Printf("[ipam] Failed to create address:%v err:%v.", address, err)
					continue
				}
			}

		}
	}

	// Set the local address space as active.
	if err = source.sink.setAddressSpace(local); err != nil {
		return err
	}

	log.Printf("[ipam] Address space successfully populated from config file")

	return err
}

func retrieveKubernetesPodIPs(node *v1.Node, desiredMaskV6 string) (*NetworkInterfaces, error) {
	var nodeCidr net.IP
	var ipnetv6 *net.IPNet
	_, desiredMask, err := net.ParseCIDR(desiredMaskV6)
	if err != nil {
		return nil, err
	}

	// get IPv6 subnet allocated to node
	for _, cidr := range node.Spec.PodCIDRs {
		nodeCidr, _, err = net.ParseCIDR(cidr)
		fmt.Println(cidr)
		if nodeCidr.To4() == nil {
			break
		}
	}

	desiredMaskSize, _ := desiredMask.Mask.Size()
	subnet := nodeCidr.String() + "/" + strconv.Itoa(desiredMaskSize)
	nodeCidr, ipnetv6, err = net.ParseCIDR(subnet)
	if err != nil {
		return nil, err
	}

	addresses := getIPsFromAddresses(nodeCidr, ipnetv6)

	networkSubnet := IPSubnet{
		Prefix: subnet,
	}

	// skip the first address
	for i := 1; i < len(addresses); i++ {
		ipaddress := IPAddress{
			IsPrimary: false,
			Address:   addresses[i].String(),
		}
		networkSubnet.IPAddresses = append(networkSubnet.IPAddresses, ipaddress)
	}

	networkInterfaces := NetworkInterfaces{
		Interfaces: []Interface{
			{
				IsPrimary: true,
				IPSubnets: []IPSubnet{
					networkSubnet,
				},
			},
		},
	}

	return &networkInterfaces, nil
}

func incIPFromIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func duplicateIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

func getIPsFromAddresses(ipv6 net.IP, ipnet *net.IPNet) []net.IP {
	ips := make([]net.IP, 0)

	for ipv6 := ipv6.Mask(ipnet.Mask); ipnet.Contains(ipv6); incIPFromIP(ipv6) {
		ips = append(ips, duplicateIP(ipv6))
	}
	return ips
}
