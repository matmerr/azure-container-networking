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

	service.lock.Lock()
	containerDetails := service.ContainerStatus[ipState.NCID]
	service.lock.Unlock()

	savedReq := containerDetails.CreateNetworkContainerRequest
	reserveResp := &cns.GetNetworkContainerResponse{
		Response:                   resp,
		NetworkContainerID:         savedReq.NetworkContainerid,
		IPConfiguration:            savedReq.IPConfiguration,
		Routes:                     savedReq.Routes,
		CnetAddressSpace:           savedReq.CnetAddressSpace,
		MultiTenancyInfo:           savedReq.MultiTenancyInfo,
		PrimaryInterfaceIdentifier: savedReq.PrimaryInterfaceIdentifier,
		LocalIPConfiguration:       savedReq.LocalIPConfiguration,
		AllowHostToNCCommunication: savedReq.AllowHostToNCCommunication,
		AllowNCToHostCommunication: savedReq.AllowNCToHostCommunication,
	}
	reserveResp.IPConfiguration.IPSubnet = ipState.IPConfig

	err = service.Listener.Encode(w, &reserveResp)
	logger.Response(service.Name, reserveResp, resp.ReturnCode, ReturnCodeToString(resp.ReturnCode), err)
}

func (service *HTTPRestService) releaseIPConfig(w http.ResponseWriter, r *http.Request) error {
	var (
		podInfo cns.KubernetesPodInfo
		req     cns.GetNetworkContainerRequest
	)

	err := service.Listener.Decode(w, r, &req)
	logger.Request(service.Name, &req, err)
	if err != nil {
		return err
	}

	defer func() {
		resp := cns.Response{}

		if err != nil {
			resp.ReturnCode = UnexpectedError
			resp.Message = err.Error()
		}

		err = service.Listener.Encode(w, &resp)
		logger.Response(service.Name, resp, resp.ReturnCode, ReturnCodeToString(resp.ReturnCode), err)
	}()

	service.lock.Lock()
	defer service.lock.Unlock()

	if service.state.OrchestratorType != cns.Kubernetes {
		err = fmt.Errorf("AllocateIPconfig API supported only for kubernetes orchestrator")
		return err
	}

	// retrieve podinfo  from orchestrator context
	if err := json.Unmarshal(req.OrchestratorContext, &podInfo); err != nil {
		return nil
	}

	ipID := service.PodIPIDByOrchestratorContext[podInfo.PodName+podInfo.PodNamespace]
	if ipID != "" {
		if _, isExist := service.PodIPConfigState[ipID]; isExist {
			return fmt.Errorf("Pod->IPIP exists but IPID to IPConfig doesn't exist")
		}
	} else {
		return nil
	}
	return nil
}

// If IPConfig is already allocated for pod, it returns that else it returns one of the available ipconfigs.
func getIPConfig(service *HTTPRestService, req cns.GetNetworkContainerRequest) (cns.ContainerIPConfigState, error) {

	service.lock.Lock()
	defer service.lock.Unlock()

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
	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContext()]
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
					ipState.State = cns.Allocated
					ipState.OrchestratorContext = req.OrchestratorContext
					return ipState, nil
				}
				return ipState, fmt.Errorf("Desired IP has already been allocated")
			}
		}
	} else {
		// return any free IPConfig
		for _, ipState = range service.PodIPConfigState {
			if ipState.State == cns.Available {
				ipState.State = cns.Allocated
				ipState.OrchestratorContext = req.OrchestratorContext
				return ipState, nil
			}
		}
		return ipState, fmt.Errorf("No more free IP's available, trigger batch")
	}

	// TODO Handle rebatching here

	return ipState, nil
}

func releaseIPConfig(service *HTTPRestService, req cns.GetNetworkContainerRequest) error {

	service.lock.Lock()
	defer service.lock.Unlock()

	var (
		err     error
		podInfo cns.KubernetesPodInfo
		isExist bool
	)

	if service.state.OrchestratorType != cns.Kubernetes {
		return fmt.Errorf("Release IPConfig API supported only for kubernetes orchestrator")
	}

	// retrieve podinfo  from orchestrator context
	if err = json.Unmarshal(req.OrchestratorContext, &podInfo); err != nil {
		return err
	}

	// check if ipconfig already allocated for this pod and return
	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContext()]
	if ipID != "" {
		if _, isExist = service.PodIPConfigState[ipID]; isExist {
			// reset state to be free
			freeIPState := service.PodIPConfigState[ipID]
			freeIPState.State = cns.Available
			freeIPState.OrchestratorContext = nil
			service.PodIPConfigState[ipID] = freeIPState
			service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContext()] = ""
			return nil
		}
		return fmt.Errorf("Pod->IPIP exists but IPID to IPConfig doesn't exist")
	}

	return err
}

/*
apiVersion: acn.azure.com/v1alpha
kind: NodeNetworkConfig
metadata:
  name: my-nnc
  namespace: kube-system
spec:
  iPsNotInUse:
    - aabbc
    - aabbcc
    - bbbb
  requestedIPCount: 15
status:
  batchSize: 23
  networkContainers:
    - ID: yo
      primaryIP: yo
      subnetID: yo
      ipAssignments:
        - name: yo
          ip: yo
      defaultGateway: yo
      netMask: yo
  releaseThresholdPercent: 30
  requestThresholdPercent: 40


*/
