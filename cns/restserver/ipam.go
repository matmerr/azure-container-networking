// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/logger"
)

func newIPConfig(ipAddress string, prefixLength uint8) cns.IPSubnet {
	return cns.IPSubnet{
		IPAddress:    ipAddress,
		PrefixLength: prefixLength,
	}
}

func newPodState(ipaddress string, prefixLength uint8, id, ncid, state string) cns.ContainerIPConfigState {
	ipconfig := newIPConfig(ipaddress, prefixLength)

	return cns.ContainerIPConfigState{
		IPConfig: ipconfig,
		ID:       id,
		NCID:     ncid,
		State:    state,
	}
}

func NewPodStateWithOrchestratorContext(ipaddress string, prefixLength uint8, id, ncid, state string, orchestratorContext cns.KubernetesPodInfo) (cns.ContainerIPConfigState, error) {
	ipconfig := newIPConfig(ipaddress, prefixLength)
	b, err := json.Marshal(orchestratorContext)
	return cns.ContainerIPConfigState{
		IPConfig:            ipconfig,
		ID:                  id,
		NCID:                ncid,
		State:               state,
		OrchestratorContext: b,
	}, err
}

func (service *HTTPRestService) AddIPConfigsToState(ipconfigs []cns.ContainerIPConfigState) {
	service.Lock()
	defer service.Unlock()
	for _, ipconfig := range ipconfigs {
		service.PodIPConfigState[ipconfig.ID] = ipconfig
	}
}

func (service *HTTPRestService) SetIPConfigAsAllocated(ipconfig cns.ContainerIPConfigState, podInfo cns.KubernetesPodInfo) (cns.ContainerIPConfigState, error) {
	service.Lock()
	defer service.Unlock()

	rawOrchestratorContext, err := json.Marshal(podInfo)
	if err != nil {
		return ipconfig, err
	}

	ipconfig.State = cns.Allocated
	ipconfig.OrchestratorContext = rawOrchestratorContext

	service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()] = ipconfig.ID
	service.PodIPConfigState[ipconfig.ID] = ipconfig
	return ipconfig, err
}

func (service *HTTPRestService) SetIPConfigAsAvailable(ipconfig cns.ContainerIPConfigState, podInfo cns.KubernetesPodInfo) {
	service.Lock()
	defer service.Unlock()
	ipconfig.State = cns.Available
	ipconfig.OrchestratorContext = nil
	service.PodIPConfigState[ipconfig.ID] = ipconfig
	service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()] = ""
	return
}

// cni -> allocate ipconfig
// 			|- fetch nc from state by constructing nc id
func (service *HTTPRestService) allocateIPConfig(w http.ResponseWriter, r *http.Request) {
	var (
		err           error
		ncrequest     cns.GetNetworkContainerRequest
		ipState       cns.ContainerIPConfigState
		returnCode    int
		returnMessage string
	)

	err = service.Listener.Decode(w, r, &ncrequest)
	logger.Request(service.Name, &ncrequest, err)
	if err != nil {
		return
	}

	// retrieve ipconfig from nc
	if ipState, err = getIPConfig(service, ncrequest); err != nil {
		returnCode = UnexpectedError
		returnMessage = fmt.Sprintf("AllocateIPConfig failed: %v", err)
	}

	resp := cns.Response{
		ReturnCode: returnCode,
		Message:    returnMessage,
	}

	reserveResp := &cns.GetNetworkContainerResponse{
		Response: resp,
	}
	reserveResp.IPConfiguration.IPSubnet = ipState.IPConfig

	err = service.Listener.Encode(w, &reserveResp)
	logger.Response(service.Name, reserveResp, resp.ReturnCode, ReturnCodeToString(resp.ReturnCode), err)
}

func (service *HTTPRestService) releaseIPConfig(w http.ResponseWriter, r *http.Request) {
	var (
		podInfo    cns.KubernetesPodInfo
		req        cns.GetNetworkContainerRequest
		statusCode int
	)
	statusCode = -1

	err := service.Listener.Decode(w, r, &req)
	logger.Request(service.Name, &req, err)
	if err != nil {
		return
	}

	defer func() {
		resp := cns.Response{}

		if err != nil {
			if statusCode < 0 {
				resp.ReturnCode = UnexpectedError
			} else {
				resp.ReturnCode = statusCode
			}

			resp.Message = err.Error()
		}

		err = service.Listener.Encode(w, &resp)
		logger.Response(service.Name, resp, resp.ReturnCode, ReturnCodeToString(resp.ReturnCode), err)
	}()

	if service.state.OrchestratorType != cns.Kubernetes {
		err = fmt.Errorf("ReleaseIPConfig API supported only for kubernetes orchestrator")
		return
	}

	// retrieve podinfo  from orchestrator context
	if err := json.Unmarshal(req.OrchestratorContext, &podInfo); err != nil {
		return
	}

	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()]
	if ipID != "" {
		if _, isExist := service.PodIPConfigState[ipID]; isExist {
			ipstate := service.PodIPConfigState[ipID]
			service.SetIPConfigAsAvailable(ipstate, podInfo)
		}
	} else {
		statusCode = NotFound
		err = fmt.Errorf("ReleaseIPConfig failed to release, no allocation found for pod")
		return
	}
	return
}

// If IPConfig is already allocated for pod, it returns that else it returns one of the available ipconfigs.
func getIPConfig(service *HTTPRestService, req cns.GetNetworkContainerRequest) (cns.ContainerIPConfigState, error) {

	var (
		podInfo cns.KubernetesPodInfo
		ipState cns.ContainerIPConfigState
		isExist bool
	)

	if service.state.OrchestratorType != cns.Kubernetes {
		return ipState, fmt.Errorf("AllocateIPconfig API supported only for kubernetes orchestrator")
	}

	// retrieve podinfo  from orchestrator context
	if err := json.Unmarshal(req.OrchestratorContext, &podInfo); err != nil {
		return ipState, err
	}

	// check if ipconfig already allocated for this pod and return
	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()]
	if ipID != "" {
		if ipState, isExist = service.PodIPConfigState[ipID]; isExist {
			return ipState, nil
		}
		return ipState, fmt.Errorf("Pod->IPIP exists but IPID to IPConfig doesn't exist")
	}

	// return desired IPConfig
	if req.DesiredIPConfig.IPAddress != "" {
		for _, ipState = range service.PodIPConfigState {
			if ipState.IPConfig.IPAddress == req.DesiredIPConfig.IPAddress {
				if ipState.State != cns.Allocated {
					service.SetIPConfigAsAllocated(ipState, podInfo)
					return ipState, nil
				}
				return ipState, fmt.Errorf("Desired IP has already been allocated")
			}
		}
	} else {
		// return any free IPConfig
		for _, ipState = range service.PodIPConfigState {
			if ipState.State == cns.Available {
				return service.SetIPConfigAsAllocated(ipState, podInfo)
			}
		}
		return ipState, fmt.Errorf("No more free IP's available, trigger batch")
	}

	// TODO Handle rebatching here

	return ipState, nil
}

func releaseIPConfig(service *HTTPRestService, req cns.GetNetworkContainerRequest) error {

	service.Lock()
	defer service.Unlock()

	var (
		err     error
		podInfo cns.KubernetesPodInfo
	)

	if service.state.OrchestratorType != cns.Kubernetes {
		return fmt.Errorf("Release IPConfig API supported only for kubernetes orchestrator")
	}

	// retrieve podinfo  from orchestrator context
	if err = json.Unmarshal(req.OrchestratorContext, &podInfo); err != nil {
		return err
	}

	// check if ipconfig already allocated for this pod and return
	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()]
	if ipID != "" {
		if _, isExist := service.PodIPConfigState[ipID]; isExist {
			// reset state to be free
			service.SetIPConfigAsAvailable(service.PodIPConfigState[ipID], podInfo)
			return nil
		}
		return fmt.Errorf("Pod->IPIP exists but IPID to IPConfig doesn't exist")
	}

	return err
}
