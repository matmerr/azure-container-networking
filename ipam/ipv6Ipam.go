package ipam

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
)

type ipv6IpamSource struct {
	name string
	sink addressConfigSink
}

func newIPv6IpamSource(options map[string]interface{}) (*ipv6IpamSource, error) {

	name := options[common.OptEnvironment].(string)
	return &ipv6IpamSource{
		name: name,
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

func loadKubernetesConfig() (kubernetes.Interface, error) {
	var filePath string

	if runtime.GOOS == windows {
		filePath = defaultWindowsKubeConfigFilePath
	} else {
		filePath = defaultLinuxKubeConfigFilePath
	}
	config, err := clientcmd.BuildConfigFromFlags("", filePath)
	client, err := kubernetes.NewForConfig(config)

	return client, err
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

func carveAddresses(node *v1.Node, desiredMaskV6 string) (*NetworkInterfaces, error) {
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
	for _, address := range addresses {

		ipaddress := IPAddress{
			IsPrimary: false,
			Address:   address.String(),
		}
		networkSubnet.IPAddresses = append(networkSubnet.IPAddresses, ipaddress)
	}

	// set address to primary
	networkSubnet.IPAddresses[0].IsPrimary = true

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

func (source *ipv6IpamSource) refresh() error {

	nodeName, err := os.Hostname()
	client, err := loadKubernetesConfig()
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	log.Printf("[ipam] Discovered CIDR's %v.", node.Spec.PodCIDRs)
	if err != nil {
		return err
	}

	file, _ := json.MarshalIndent(node.Spec, "", " ")
	filepath := "/tmp/nodespec.json"
	err = ioutil.WriteFile(filepath, file, 0644)

	return err
}
