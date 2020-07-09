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

func NewPodState(ipaddress string, prefixLength uint8, id, ncid, state string) *cns.ContainerIPConfigState {
	ipconfig := newIPConfig(ipaddress, prefixLength)

	return &cns.ContainerIPConfigState{
		IPConfig: ipconfig,
		ID:       id,
		NCID:     ncid,
		State:    state,
	}
}

func NewPodStateWithOrchestratorContext(ipaddress string, prefixLength uint8, id, ncid, state string, orchestratorContext cns.KubernetesPodInfo) (*cns.ContainerIPConfigState, error) {
	ipconfig := newIPConfig(ipaddress, prefixLength)
	b, err := json.Marshal(orchestratorContext)
	return &cns.ContainerIPConfigState{
		IPConfig:            ipconfig,
		ID:                  id,
		NCID:                ncid,
		State:               state,
		OrchestratorContext: b,
	}, err
}

//AddIPConfigsToState takes a lock on the service object, and will add an array of ipconfigs to the CNS Service.
//Used to add IPConfigs to the CNS pool, specifically in the scenario of rebatching.
func (service *HTTPRestService) AddIPConfigsToState(ipconfigs []*cns.ContainerIPConfigState) error {
	service.Lock()
	defer service.Unlock()

	for i, ipconfig := range ipconfigs {
		service.PodIPConfigState[ipconfig.ID] = ipconfig
		if ipconfig.State == cns.Allocated {
			var podInfo cns.KubernetesPodInfo
			err := json.Unmarshal(ipconfig.OrchestratorContext, &podInfo)

			// if batch request failed, remove added ipconfigs and return
			if err != nil {
				errRemove := service.RemoveIPConfigsFromState(ipconfigs[0:i])
				if errRemove != nil {
					return fmt.Errorf("Failed remove IPConfig after AddIpConfigs: %v", err)
				}
				return fmt.Errorf("Failed to add IPConfig to state: %+v with error: %v", ipconfig, err)
			}
			service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()] = ipconfig.ID
		}
	}
	return nil
}

//RemoveIPConfigsFromState takes a lock on the service object, and will remove an array of ipconfigs to the CNS Service.
//Used to add IPConfigs to the CNS pool, specifically in the scenario of rebatching.
func (service *HTTPRestService) RemoveIPConfigsFromState(ipconfigs []*cns.ContainerIPConfigState) error {
	service.Lock()
	defer service.Unlock()

	for _, ipconfig := range ipconfigs {
		service.PodIPConfigState[ipconfig.ID] = nil
		var podInfo cns.KubernetesPodInfo
		err := json.Unmarshal(ipconfig.OrchestratorContext, &podInfo)

		// if batch delete failed return
		if err != nil {
			return err
		}

		if ipconfig.State == cns.Allocated {
			service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()] = ""
		}
	}
	return nil
}

//SetIPConfigAsAllocated takes a lock of the service, and sets the ipconfig in the CNS stateas Allocated
func (service *HTTPRestService) SetIPConfigAsAllocated(ipconfig *cns.ContainerIPConfigState, podInfo cns.KubernetesPodInfo, marshalledOrchestratorContext json.RawMessage) *cns.ContainerIPConfigState {
	ipconfig.State = cns.Allocated
	ipconfig.OrchestratorContext = marshalledOrchestratorContext

	service.Lock()
	defer service.Unlock()
	service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()] = ipconfig.ID
	service.PodIPConfigState[ipconfig.ID] = ipconfig
	return service.PodIPConfigState[ipconfig.ID]
}

////SetIPConfigAsAllocated takes a lock of the service, and sets the ipconfig in the CNS stateas Available
func (service *HTTPRestService) SetIPConfigAsAvailable(ipconfig *cns.ContainerIPConfigState, podInfo cns.KubernetesPodInfo) {
	ipconfig.State = cns.Available
	ipconfig.OrchestratorContext = nil
	service.Lock()
	defer service.Unlock()
	service.PodIPConfigState[ipconfig.ID] = ipconfig
	service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()] = ""
	return
}

// cni -> allocate ipconfig
// 			|- fetch nc from state by constructing nc id
func (service *HTTPRestService) requestIPConfigHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err           error
		ncrequest     cns.GetNetworkContainerRequest
		ipState       *cns.ContainerIPConfigState
		returnCode    int
		returnMessage string
	)

	err = service.Listener.Decode(w, r, &ncrequest)
	logger.Request(service.Name, &ncrequest, err)
	if err != nil {
		return
	}

	// retrieve ipconfig from nc
	if ipState, err = requestIPConfig(service, ncrequest); err != nil {
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

func (service *HTTPRestService) releaseIPConfigHandler(w http.ResponseWriter, r *http.Request) {
	var (
		podInfo    cns.KubernetesPodInfo
		req        cns.GetNetworkContainerRequest
		statusCode int
	)
	statusCode = UnexpectedError

	err := service.Listener.Decode(w, r, &req)
	logger.Request(service.Name, &req, err)
	if err != nil {
		return
	}

	defer func() {
		resp := cns.Response{}

		if err != nil {
			resp.ReturnCode = statusCode
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
	if err = json.Unmarshal(req.OrchestratorContext, &podInfo); err != nil {
		return
	}

	service.RLock()
	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()]
	if ipID != "" {
		if _, isExist := service.PodIPConfigState[ipID]; isExist {
			ipstate := service.PodIPConfigState[ipID]
			service.RUnlock()
			service.SetIPConfigAsAvailable(ipstate, podInfo)
		}
	} else {
		service.RUnlock()
		statusCode = NotFound
		err = fmt.Errorf("ReleaseIPConfig failed to release, no allocation found for pod")
		return
	}
	return
}

// If IPConfig is already allocated for pod, it returns that else it returns one of the available ipconfigs.
func requestIPConfig(service *HTTPRestService, req cns.GetNetworkContainerRequest) (*cns.ContainerIPConfigState, error) {

	var (
		podInfo cns.KubernetesPodInfo
		ipState *cns.ContainerIPConfigState
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
	service.RLock()
	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()]
	if ipID != "" {
		if ipState, isExist = service.PodIPConfigState[ipID]; isExist {
			return ipState, nil
		}
		return ipState, fmt.Errorf("Pod->IPIP exists but IPID to IPConfig doesn't exist")
	}
	service.RUnlock()

	// return desired IPConfig
	if req.DesiredIPConfig.IPAddress != "" {
		for _, ipState = range service.PodIPConfigState {
			if ipState.IPConfig.IPAddress == req.DesiredIPConfig.IPAddress {
				if ipState.State == cns.Available {
					return service.SetIPConfigAsAllocated(ipState, podInfo, req.OrchestratorContext), nil
				}
				return ipState, fmt.Errorf("Desired IP has already been allocated")
			}
		}
		return ipState, fmt.Errorf("Requested IP not found in pool")
	} else {
		// return any free IPConfig
		for _, ipState = range service.PodIPConfigState {
			if ipState.State == cns.Available {
				return service.SetIPConfigAsAllocated(ipState, podInfo, req.OrchestratorContext), nil
			}
		}
		return ipState, fmt.Errorf("No more free IP's available, trigger batch")
	}
}

// this is called by the releaseIPConfig Handler, it releases an IPConfig to be allocated elsewhere,
// but still keeps the ipconfig in the CNS state
func releaseIPConfig(service *HTTPRestService, req cns.GetNetworkContainerRequest) error {

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
	service.RLock()
	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()]
	if ipID != "" {
		if _, isExist := service.PodIPConfigState[ipID]; isExist {
			service.RUnlock()
			// reset state to be free
			service.SetIPConfigAsAvailable(service.PodIPConfigState[ipID], podInfo)
			return nil
		}
		service.RUnlock()
		return fmt.Errorf("Pod->IPIP exists but IPID to IPConfig doesn't exist")
	}
	service.RUnlock()
	return err
}
