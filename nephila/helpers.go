package nephila

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func StartFlannel(flannelConfig FlannelDNCConfig) error {

	return nil
}

// Handles retrieval of over
func GetFlannelConfiguration() (*FlannelNodeConfig, error) {
	fp, err := os.Open("/var/run/flannel/subnet.env")

	var flannel FlannelNodeConfig

	if err != nil {
		return nil, fmt.Errorf("[Azure CNS Flannel] Error. Loading Flannel subnet file failed with error: %v", err)
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
				return nil, fmt.Errorf("[Azure CNS Flannel] Error. Flannel Network env failed to parse node subnet prefix.")
			}
			flannel.OverlaySubnet.PrefixLength = uint8(prefix)
		} else {
			return nil, fmt.Errorf("[Azure CNS Flannel] Error. Flannel Subnet env not found.")
		}

		// read allocatable space for the subnet on node
		if v, exists := fenvs["FLANNEL_SUBNET"]; exists {
			props := strings.Split(v, "/") // ex 169.254.22.1/24
			flannel.NodeSubnet.IPAddress = props[0]
			prefix, err := strconv.ParseInt(props[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("[Azure CNS Flannel] Error. Flannel Network env failed to parse node subnet prefix.")
			}
			flannel.NodeSubnet.PrefixLength = uint8(prefix)
		} else {
			return nil, fmt.Errorf("[Azure CNS Flannel] Error. Flannel Subnet env not found.")
		}

		if v, exists := fenvs["FLANNEL_MTU"]; exists {
			mtu, err := strconv.ParseInt(v, 10, 32)
			if err != nil {

				return nil, errors.New("[Azure CNS Flannel] Error. Flannel Network env failed to parse node MTU.")
			}
			flannel.InterfaceMTU = mtu
		} else {
			return nil, fmt.Errorf("[Azure CNS Flannel] Error. Flannel MTU env not found.")
		}

		if v, exists := fenvs["FLANNEL_IPMASQ"]; exists {
			ipmasq, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("[Azure CNS Flannel] Error. Flannel Network env failed to parse IPMASQ.")
			}
			flannel.IPMASQ = ipmasq
		} else {
			return nil, fmt.Errorf("[Azure CNS Flannel] Error. Flannel IPMASQ env not found.")
		}
	}

	return &flannel, nil
}
