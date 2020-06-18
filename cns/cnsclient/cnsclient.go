package cnsclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/log"

	cniTypesCurr "github.com/containernetworking/cni/pkg/types/current"
)

// CNSClient specifies a client to connect to Ipam Plugin.
type CNSClient struct {
	connectionURL string
}

const (
	defaultCnsURL = "http://localhost:10090"
)

var (
	cnsClient *CNSClient
)

// InitCnsClient initializes new cns client and returns the object
func InitCnsClient(url string) (*CNSClient, error) {
	if cnsClient == nil {
		if url == "" {
			url = defaultCnsURL
		}

		cnsClient = &CNSClient{
			connectionURL: url,
		}
	}

	return cnsClient, nil
}

// GetCnsClient returns the cns client object
func GetCnsClient() (*CNSClient, error) {
	var err error

	if cnsClient == nil {
		err = fmt.Errorf("[Azure CNSClient] CNS Client not initialized")
	}

	return cnsClient, err
}

// GetNetworkConfiguration Request to get network config.
func (cnsClient *CNSClient) GetNetworkConfiguration(orchestratorContext []byte) (*cns.GetNetworkContainerResponse, error) {
	var body bytes.Buffer

	httpc := &http.Client{}
	url := cnsClient.connectionURL + cns.GetNetworkContainerByOrchestratorContext
	log.Printf("GetNetworkConfiguration url %v", url)

	payload := &cns.GetNetworkContainerRequest{
		OrchestratorContext: orchestratorContext,
	}

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		log.Errorf("encoding json failed with %v", err)
		return nil, err
	}

	res, err := httpc.Post(url, "application/json", &body)
	if err != nil {
		log.Errorf("[Azure CNSClient] HTTP Post returned error %v", err.Error())
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("[Azure CNSClient] GetNetworkConfiguration invalid http status code: %v", res.StatusCode)
		log.Errorf(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	var resp cns.GetNetworkContainerResponse

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		log.Errorf("[Azure CNSClient] Error received while parsing GetNetworkConfiguration response resp:%v err:%v", res.Body, err.Error())
		return nil, err
	}

	if resp.Response.ReturnCode != 0 {
		log.Errorf("[Azure CNSClient] GetNetworkConfiguration received error response :%v", resp.Response.Message)
		return nil, fmt.Errorf(resp.Response.Message)
	}

	return &resp, nil
}

// CreateHostNCApipaEndpoint creates an endpoint in APIPA network for host container connectivity.
func (cnsClient *CNSClient) CreateHostNCApipaEndpoint(
	networkContainerID string) (string, error) {
	var (
		err  error
		body bytes.Buffer
	)

	httpc := &http.Client{}
	url := cnsClient.connectionURL + cns.CreateHostNCApipaEndpointPath
	log.Printf("CreateHostNCApipaEndpoint url: %v for NC: %s", url, networkContainerID)

	payload := &cns.CreateHostNCApipaEndpointRequest{
		NetworkContainerID: networkContainerID,
	}

	if err = json.NewEncoder(&body).Encode(payload); err != nil {
		log.Errorf("encoding json failed with %v", err)
		return "", err
	}

	res, err := httpc.Post(url, "application/json", &body)
	if err != nil {
		log.Errorf("[Azure CNSClient] HTTP Post returned error %v", err.Error())
		return "", err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("[Azure CNSClient] CreateHostNCApipaEndpoint: Invalid http status code: %v",
			res.StatusCode)
		log.Errorf(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	var resp cns.CreateHostNCApipaEndpointResponse

	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		log.Errorf("[Azure CNSClient] Error parsing CreateHostNCApipaEndpoint response resp: %v err: %v",
			res.Body, err.Error())
		return "", err
	}

	if resp.Response.ReturnCode != 0 {
		log.Errorf("[Azure CNSClient] CreateHostNCApipaEndpoint received error response :%v", resp.Response.Message)
		return "", fmt.Errorf(resp.Response.Message)
	}

	return resp.EndpointID, nil
}

// DeleteHostNCApipaEndpoint deletes the endpoint in APIPA network created for host container connectivity.
func (cnsClient *CNSClient) DeleteHostNCApipaEndpoint(networkContainerID string) error {
	var body bytes.Buffer

	httpc := &http.Client{}
	url := cnsClient.connectionURL + cns.DeleteHostNCApipaEndpointPath
	log.Printf("DeleteHostNCApipaEndpoint url: %v for NC: %s", url, networkContainerID)

	payload := &cns.DeleteHostNCApipaEndpointRequest{
		NetworkContainerID: networkContainerID,
	}

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		log.Errorf("encoding json failed with %v", err)
		return err
	}

	res, err := httpc.Post(url, "application/json", &body)
	if err != nil {
		log.Errorf("[Azure CNSClient] HTTP Post returned error %v", err.Error())
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("[Azure CNSClient] DeleteHostNCApipaEndpoint: Invalid http status code: %v",
			res.StatusCode)
		log.Errorf(errMsg)
		return fmt.Errorf(errMsg)
	}

	var resp cns.DeleteHostNCApipaEndpointResponse

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		log.Errorf("[Azure CNSClient] Error parsing DeleteHostNCApipaEndpoint response resp: %v err: %v",
			res.Body, err.Error())
		return err
	}

	if resp.Response.ReturnCode != 0 {
		log.Errorf("[Azure CNSClient] DeleteHostNCApipaEndpoint received error response :%v", resp.Response.Message)
		return fmt.Errorf(resp.Response.Message)
	}

	return nil
}

func (cnsClient *CNSClient) RequestIPAddress(orchestratorContext []byte) (*cniTypesCurr.Result, *cniTypesCurr.Result, error) {
	var (
		result   cniTypesCurr.Result
		resultV6 cniTypesCurr.Result
		err      error
	)

	defer func() {
		if err != nil {
			cnsClient.ReleaseIPAddress(orchestratorContext)
		}
	}()

	var body bytes.Buffer

	httpc := &http.Client{}
	url := cnsClient.connectionURL + cns.AllocateIPConfig
	log.Printf("RequestIPAddress url %v", url)

	payload := &cns.GetNetworkContainerRequest{
		OrchestratorContext: orchestratorContext,
	}

	err = json.NewEncoder(&body).Encode(payload)
	if err != nil {
		log.Errorf("encoding json failed with %v", err)
		return nil, nil, err
	}

	res, err := httpc.Post(url, "application/json", &body)
	if err != nil {
		log.Errorf("[Azure CNSClient] HTTP Post returned error %v", err.Error())
		return nil, nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("[Azure CNSClient] RequestIPAddress invalid http status code: %v", res.StatusCode)
		log.Errorf(errMsg)
		return nil, nil, fmt.Errorf(errMsg)
	}

	var resp cns.GetNetworkContainerResponse

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		log.Errorf("[Azure CNSClient] Error received while parsing RequestIPAddress response resp:%v err:%v", res.Body, err.Error())
		return nil, nil, err
	}

	if resp.Response.ReturnCode != 0 {
		log.Errorf("[Azure CNSClient] RequestIPAddress received error response :%v", resp.Response.Message)
		return nil, nil, fmt.Errorf(resp.Response.Message)
	}

	// set result ipconfig from CNS Response Body
	prefix := strconv.Itoa(int(resp.IPConfiguration.IPSubnet.PrefixLength))
	_, ipnet, err := net.ParseCIDR(resp.IPConfiguration.IPSubnet.IPAddress + "/" + prefix)
	if err != nil {
		return nil, nil, err
	}

	result.IPs = make([]*cniTypesCurr.IPConfig, 1)
	result.IPs[0].Address = *ipnet

	return &result, &resultV6, err
}

func (cnsClient *CNSClient) ReleaseIPAddress(orchestratorContext []byte) {

	return
}
