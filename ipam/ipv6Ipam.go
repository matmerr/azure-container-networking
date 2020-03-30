package ipam

import (
	"context"
	"errors"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Masterminds/semver"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultWindowsKubePath           = `c:\k\`
	defaultWindowsKubeConfigFilePath = defaultWindowsKubePath + `config`
	defaultLinuxKubeConfigFilePath   = "/var/lib/kubelet/kubeconfig"
	nodeSubnetMask                   = "/120"
	k8sMajorVerForNewPolicyDef       = "1"
	k8sMinorVerForNewPolicyDef       = "16"
)

// regex to get minor version
var re = regexp.MustCompile("[0-9]+")

type ipv6IpamSource struct {
	name            string
	nodeHostname    string
	kubeConfigPath  string
	kubeClient      kubernetes.Interface
	kubeNode        *v1.Node
	subnetRetrieved bool
	sink            addressConfigSink
}

func newIPv6IpamSource(options map[string]interface{}) (*ipv6IpamSource, error) {
	var kubeConfigPath string
	name := options[common.OptEnvironment].(string)

	if runtime.GOOS == windows {
		kubeConfigPath = defaultWindowsKubeConfigFilePath
	} else {
		kubeConfigPath = defaultLinuxKubeConfigFilePath
	}

	nodeName, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	return &ipv6IpamSource{
		name:           name,
		nodeHostname:   nodeName,
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

func (source *ipv6IpamSource) loadKubernetesConfig() (kubernetes.Interface, error) {

	config, err := clientcmd.BuildConfigFromFlags("", source.kubeConfigPath)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)

	minimumVersion := &version.Info{
		Major: k8sMajorVerForNewPolicyDef,
		Minor: k8sMinorVerForNewPolicyDef,
	}

	serverVersion, err := client.ServerVersion()

	isNewew := CompareK8sVer(serverVersion, minimumVersion)

	if isNewew <= 0 {
		return nil, errors.New("Incompatible Kubernetes version for dual stack")
	}

	return client, err
}

// CompareK8sVer compares two k8s versions.
// returns -1, 0, 1 if firstVer smaller, equals, bigger than secondVer respectively.
// returns -2 for error.
func CompareK8sVer(firstVer *version.Info, secondVer *version.Info) int {
	v1Minor := re.FindAllString(firstVer.Minor, -1)
	if len(v1Minor) < 1 {
		return -2
	}
	v1, err := semver.NewVersion(firstVer.Major + "." + v1Minor[0])
	if err != nil {
		return -2
	}
	v2Minor := re.FindAllString(secondVer.Minor, -1)
	if len(v2Minor) < 1 {
		return -2
	}
	v2, err := semver.NewVersion(secondVer.Major + "." + v2Minor[0])
	if err != nil {
		return -2
	}

	return v1.Compare(v2)
}

func (source *ipv6IpamSource) refresh() error {
	if source == nil {
		return errors.New("ipv6ipam is nil")
	}

	if source.subnetRetrieved {
		return nil
	}

	if source.kubeClient == nil {
		kubeClient, err := source.loadKubernetesConfig()
		source.kubeClient = kubeClient
		if err != nil {
			return err
		}
	}

	kubeNode, err := source.kubeClient.CoreV1().Nodes().Get(context.TODO(), source.nodeHostname, metav1.GetOptions{})
	source.kubeNode = kubeNode
	if err != nil {
		return err
	}

	log.Printf("[ipam] Discovered CIDR's %v.", source.kubeNode.Spec.PodCIDRs)

	// Query the list of Kubernetes Pod IPs
	interfaceIPs, err := retrieveKubernetesPodIPs(source.kubeNode, nodeSubnetMask)

	// Configure the local default address space.
	local, err := source.sink.newAddressSpace(LocalDefaultAddressSpaceId, LocalScope)
	if err != nil {
		return err
	}

	for _, i := range interfaceIPs.Interfaces {
		for index, s := range i.IPSubnets {
			_, subnet, err := net.ParseCIDR(s.Prefix)
			ifaceName := "azure-" + strconv.Itoa(index)
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

	source.subnetRetrieved = true
	log.Printf("[ipam] Address space successfully populated from config file")

	return err
}

func retrieveKubernetesPodIPs(node *v1.Node, subnetMaskBitSize string) (*NetworkInterfaces, error) {
	var nodeCidr net.IP
	var ipnetv6 *net.IPNet

	// get IPv6 subnet allocated to node
	for _, cidr := range node.Spec.PodCIDRs {
		nodeCidr, _, _ = net.ParseCIDR(cidr)
		if nodeCidr.To4() == nil {
			break
		}
	}

	subnet := nodeCidr.String() + subnetMaskBitSize
	nodeCidr, ipnetv6, err := net.ParseCIDR(subnet)
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

// increment the IP
func incrementIP(ip net.IP) {
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

	for ipv6 := ipv6.Mask(ipnet.Mask); ipnet.Contains(ipv6); incrementIP(ipv6) {
		ips = append(ips, duplicateIP(ipv6))
	}
	return ips
}
