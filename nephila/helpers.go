package nephila

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/flannel/pkg/ip"
)

const (
	flannelKeyPath = "/coreos.com/network/config"
	vxlan          = "vxlan"
)

type flannelEtcdBackendConfig struct {
	Type string
}

type flannelEtcdConfig struct {
	Network   ip.IP4Net
	SubnetLen uint
	Backend   flannelEtcdBackendConfig
}

func SetFlannelKey(flannelDNCConfig FlannelDNCConfig) error {
	pip := net.ParseIP(flannelDNCConfig.OverlaySubnet.IPAddress)
	if pip == nil {
		return fmt.Errorf("Failed to parse flannel overlay IP: %v", flannelDNCConfig.OverlaySubnet.IPAddress)
	}
	fetcd := flannelEtcdConfig{
		Network: ip.IP4Net{
			IP:        ip.FromIP(pip),
			PrefixLen: uint(flannelDNCConfig.OverlaySubnet.PrefixLength),
		},
		SubnetLen: uint(flannelDNCConfig.PerNodePrefixLength),
		Backend: flannelEtcdBackendConfig{
			Type: vxlan,
		},
	}

	b, err := json.Marshal(fetcd)
	value := string(b)

	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}
	c, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("Failed to create new etcd client with error %s", err)
	}
	kapi := client.NewKeysAPI(c)
	// set "/foo" key with "bar" value
	log.Printf("[Azure CNS Nephila: Flannel] Setting %s key in etcd with %s value.", flannelKeyPath, value)

	resp, err := kapi.Set(context.Background(), flannelKeyPath, value, nil)
	if err != nil {
		return fmt.Errorf("Failed to set keys in etcd with error: %s", err)
	}
	log.Printf("[Azure CNS Nephila: Flannel] Set Flannel config in etcd with response %v.", resp)
	return nil
}

// Handles retrieval of over
func GetFlannelConfiguration() (*FlannelNodeConfig, error) {
	fp, err := os.Open("/var/run/flannel/subnet.env")

	var flannel FlannelNodeConfig

	if err != nil {
		return nil, fmt.Errorf("Loading Flannel subnet file failed with error: %v", err)
	} else {
		defer fp.Close()

		fenvs := make(map[string]string)
		sr := bufio.NewScanner(fp)
		for sr.Scan() {
			ev := strings.Split(sr.Text(), "=")
			fenvs[ev[0]] = ev[1]
		}

		// read allocatable space for the overlay
		if v, exists := fenvs["FLANNEL_NETWORK"]; exists {
			props := strings.Split(v, "/") // ex 169.254.22.1/24
			flannel.OverlaySubnet.IPAddress = props[0]
			prefix, err := strconv.ParseInt(props[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("Flannel Network env failed to parse node subnet prefix.")
			}
			flannel.OverlaySubnet.PrefixLength = uint8(prefix)
		} else {
			return nil, fmt.Errorf("Flannel Subnet env not found.")
		}

		// read allocatable space for the subnet on node
		if v, exists := fenvs["FLANNEL_SUBNET"]; exists {
			props := strings.Split(v, "/") // ex 169.254.22.1/24
			flannel.NodeSubnet.IPAddress = props[0]
			prefix, err := strconv.ParseInt(props[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("Flannel Network env failed to parse node subnet prefix.")
			}
			flannel.NodeSubnet.PrefixLength = uint8(prefix)
		} else {
			return nil, fmt.Errorf("Flannel Subnet env not found.")
		}

		if v, exists := fenvs["FLANNEL_MTU"]; exists {
			mtu, err := strconv.ParseInt(v, 10, 32)
			if err != nil {

				return nil, errors.New("Flannel Network env failed to parse node MTU.")
			}
			flannel.InterfaceMTU = mtu
		} else {
			return nil, fmt.Errorf("Flannel MTU env not found.")
		}

		if v, exists := fenvs["FLANNEL_IPMASQ"]; exists {
			ipmasq, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("Error. Flannel Network env failed to parse IPMASQ.")
			}
			flannel.IPMASQ = ipmasq
		} else {
			return nil, fmt.Errorf("Error. Flannel IPMASQ env not found.")
		}
	}

	return &flannel, nil
}
