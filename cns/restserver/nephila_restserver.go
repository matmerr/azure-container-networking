package restserver

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/log"
)

func StartFlannel(flannelConfig cns.FlannelDNCConfig) error {

	return nil
}

// Handles retrieval of over
func GetFlannelConfiguration() (*cns.FlannelNodeConfig, error) {
	fp, err := os.Open("/var/run/flannel/subnet.env")

	var flannel cns.FlannelNodeConfig

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

func (service *httpRestService) setNephilaConfig(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] setNephilaConfig")

	var req cns.NephilaDNCConfig
	var res cns.NephilaConfigResponse

	returnMessage := ""
	returnCode := 0

	err := service.Listener.Decode(w, r, &req)
	if err != nil {
		return
	}

	service.lock.Lock()

	switch req.Type {
	case cns.Flannel:
		service.state.NephilaType = cns.Flannel
		// Start Flannel
		err := StartFlannel(req.Config)
		if err != nil {
			returnCode = UnexpectedError
			returnMessage = fmt.Sprintf("[Azure CNS Nephila] Failed to set Flannel config with error: %s", err.Error())
			break
		}

		// Get the env's flannel has set
		flannelConf, err := GetFlannelConfiguration()

		if err != nil {
			returnCode = UnexpectedError
			returnMessage = fmt.Sprintf("[Azure CNS Nephila] Failed to get Flannel config with error: %s", err.Error())
			break
		}
		res.NodeConfig.Config = *flannelConf

		service.saveState()
		break
	default:
		service.state.NephilaType = cns.Disabled
		service.saveState()
		break
	}

	service.lock.Unlock()

	resp := cns.Response{
		ReturnCode: returnCode,
		Message:    returnMessage,
	}

	// add cns response to Nephila config response
	res.Response = resp

	err = service.Listener.Encode(w, &res)
	log.Response(service.Name, resp, err)
}
