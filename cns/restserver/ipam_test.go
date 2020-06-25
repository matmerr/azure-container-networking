// Copyright 2020 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/common"
)

var (
	testNCID = "06867cf3-332d-409d-8819-ed70d2c116b0"

	testIP1      = "10.0.0.1"
	testPod1GUID = "898fb8f1-f93e-4c96-9c31-6b89098949a3"
	testPod1Info = cns.KubernetesPodInfo{
		PodName:      "testpod1",
		PodNamespace: "testpod1namespace",
	}

	testIP2      = "10.0.0.2"
	testPod2GUID = "b21e1ee1-fb7e-4e6d-8c68-22ee5049944e"
	testPod2Info = cns.KubernetesPodInfo{
		PodName:      "testpod2",
		PodNamespace: "testpod2namespace",
	}

	testPod3GUID = "718e04ac-5a13-4dce-84b3-040accaa9b41"
	testPod3Info = cns.KubernetesPodInfo{
		PodName:      "testpod3",
		PodNamespace: "testpod3namespace",
	}
)

func getTestService() *HTTPRestService {
	var config common.ServiceConfig
	httpsvc, _ := NewHTTPRestService(&config)
	svc := httpsvc.(*HTTPRestService)
	svc.state.OrchestratorType = cns.Kubernetes

	return svc
}

// Want first IP
func TestGetAvailableIPConfig(t *testing.T) {
	svc := getTestService()

	desiredState := newPodState(testIP1, 24, testPod1GUID, testNCID, cns.Available)
	svc.PodIPConfigState[testPod1GUID] = desiredState

	req := cns.GetNetworkContainerRequest{}
	b, _ := json.Marshal(testPod1Info)
	req.OrchestratorContext = b

	actualstate, err := getIPConfig(svc, req)
	if err != nil {
		t.Fatal("Expected IP retrieval to be nil")
	}

	desiredState.State = cns.Allocated
	desiredState.OrchestratorContext = b
	if reflect.DeepEqual(desiredState, actualstate) != true {
		t.Fatalf("Desired state not matching actual state, expected: %+v, actual: %+v", desiredState, actualstate)
	}
}

// First IP is already assigned to a pod, want second IP
func TestGetNextAvailableIPConfig(t *testing.T) {
	svc := getTestService()

	// Add already allocated pod ip to state
	svc.PodIPIDByOrchestratorContext[testPod1Info.GetOrchestratorContext()] = testPod1GUID
	state1, _ := NewPodStateWithOrchestratorContext(testIP1, 24, testPod1GUID, testNCID, cns.Allocated, testPod1Info)
	svc.PodIPConfigState[state1.ID] = state1

	state2 := newPodState(testIP2, 24, testPod2GUID, testNCID, cns.Available)
	svc.PodIPConfigState[state2.ID] = state2

	req := cns.GetNetworkContainerRequest{}
	b, _ := json.Marshal(testPod2Info)
	req.OrchestratorContext = b

	actualstate, err := getIPConfig(svc, req)
	if err != nil {
		t.Fatalf("Expected IP retrieval to be nil: %+v", err)
	}
	// want second available Pod IP State as first has been allocated
	desiredState, _ := NewPodStateWithOrchestratorContext(testIP2, 24, testPod2GUID, testNCID, cns.Allocated, testPod2Info)

	if reflect.DeepEqual(desiredState, actualstate) != true {
		t.Fatalf("Desired state not matching actual state, expected: %+v, actual: %+v", desiredState, actualstate)
	}
}

func TestGetAlreadyAllocatedIPConfigForSamePod(t *testing.T) {
	svc := getTestService()

	// Add Allocated Pod IP to state
	svc.PodIPIDByOrchestratorContext[testPod1Info.GetOrchestratorContext()] = testPod1GUID
	desiredState, _ := NewPodStateWithOrchestratorContext(testIP1, 24, testPod1GUID, testNCID, cns.Allocated, testPod1Info)
	svc.PodIPConfigState[desiredState.ID] = desiredState

	req := cns.GetNetworkContainerRequest{}
	b, _ := json.Marshal(testPod1Info)
	req.OrchestratorContext = b

	actualstate, err := getIPConfig(svc, req)
	if err != nil {
		t.Fatalf("Expected not error: %+v", err)
	}

	desiredState.State = cns.Allocated
	if reflect.DeepEqual(desiredState, actualstate) != true {
		t.Fatalf("Desired state not matching actual state, expected: %+v, actual: %+v", desiredState, actualstate)
	}
}

func TestGetDesiredIPConfigWithSpecfiedIP(t *testing.T) {
	svc := getTestService()

	// Add Available Pod IP to state
	desiredState := newPodState(testIP1, 24, testPod1GUID, testNCID, cns.Available)
	svc.PodIPConfigState[desiredState.ID] = desiredState

	req := cns.GetNetworkContainerRequest{}
	b, _ := json.Marshal(testPod2Info)
	req.OrchestratorContext = b
	req.DesiredIPConfig = newIPConfig(testIP2, 24)

	actualstate, err := getIPConfig(svc, req)
	if err != nil {
		t.Fatalf("Expected IP retrieval to be nil: %+v", err)
	}

	desiredState.OrchestratorContext = b
	desiredState.State = cns.Allocated
	if reflect.DeepEqual(desiredState, actualstate) != true {
		t.Fatalf("Desired state not matching actual state, expected: %+v, actual: %+v", desiredState, actualstate)
	}
}

func TestFailToGetDesiredIPConfigWithAlreadyAllocatedSpecfiedIP(t *testing.T) {
	svc := getTestService()

	// set state as already allocated
	svc.PodIPIDByOrchestratorContext[testPod1Info.GetOrchestratorContext()] = testPod1GUID
	desiredState, _ := NewPodStateWithOrchestratorContext(testIP1, 24, testPod1GUID, testNCID, cns.Allocated, testPod1Info)
	svc.PodIPConfigState[desiredState.ID] = desiredState

	// request the already allocated ip with a new context
	req := cns.GetNetworkContainerRequest{}
	b, _ := json.Marshal(testPod2Info)
	req.OrchestratorContext = b
	req.DesiredIPConfig = newIPConfig(testIP1, 24)

	_, err := getIPConfig(svc, req)
	if err == nil {
		t.Fatalf("Expected failure requesting already IP: %+v", err)
	}
}

func TestFailToGetIPWhenAllIPsAreAllocated(t *testing.T) {
	svc := getTestService()

	// set state as already allocated
	svc.PodIPIDByOrchestratorContext[testPod1Info.GetOrchestratorContext()] = testPod1GUID
	state1, _ := NewPodStateWithOrchestratorContext(testIP1, 24, testPod1GUID, testNCID, cns.Allocated, testPod1Info)
	svc.PodIPConfigState[state1.ID] = state1

	// set state as already allocated
	svc.PodIPIDByOrchestratorContext[testPod2Info.GetOrchestratorContext()] = testPod1GUID
	state2, _ := NewPodStateWithOrchestratorContext(testIP2, 24, testPod2GUID, testNCID, cns.Allocated, testPod2Info)
	svc.PodIPConfigState[state2.ID] = state2

	// request the already allocated ip with a new context
	req := cns.GetNetworkContainerRequest{}
	b, _ := json.Marshal(testPod3Info)
	req.OrchestratorContext = b

	_, err := getIPConfig(svc, req)
	if err == nil {
		t.Fatalf("Expected failure requesting IP when there are no more IP's: %+v", err)
	}
}

// 10.0.0.1 = PodInfo1
// Request 10.0.0.1 with PodInfo2 (Fail)
// Release PodInfo1
// Request 10.0.0.1 with PodInfo2 (Success)
func TestRequestThenReleaseThenRequestAgain(t *testing.T) {
	svc := getTestService()

	// set state as already allocated
	state1, _ := NewPodStateWithOrchestratorContext(testIP1, 24, testPod1GUID, testNCID, cns.Allocated, testPod1Info)
	ipconfigs := []cns.ContainerIPConfigState{
		state1,
	}
	svc.AddIPConfigsToState(ipconfigs)
	svc.SetIPConfigAsAllocated(ipconfigs[0], testPod1Info)

	desiredIPConfig := newIPConfig(testIP1, 24)

	// Use TestPodInfo2 to request TestIP1, which has already been allocated
	req := cns.GetNetworkContainerRequest{}
	b, _ := json.Marshal(testPod2Info)
	req.OrchestratorContext = b
	req.DesiredIPConfig = desiredIPConfig

	_, err := getIPConfig(svc, req)
	if err == nil {
		t.Fatal("Expected failure requesting IP when there are no more IP's")
	}

	// Release Test Pod 1
	req2 := cns.GetNetworkContainerRequest{}
	b, _ = json.Marshal(testPod1Info)
	req2.OrchestratorContext = b
	err = releaseIPConfig(svc, req2)
	if err != nil {
		t.Fatalf("Unexpected failure releasing IP: %+v", err)
	}

	// Rerequest
	req = cns.GetNetworkContainerRequest{}
	b, _ = json.Marshal(testPod2Info)
	req.OrchestratorContext = b
	req.DesiredIPConfig = desiredIPConfig
	actualstate, err := getIPConfig(svc, req)

	if err != nil {
		t.Fatalf("Expected IP retrieval to be nil: %+v", err)
	}

	// want first available Pod IP State
	state1.IPConfig = desiredIPConfig
	state1.OrchestratorContext = b

	if reflect.DeepEqual(state1, actualstate) != true {
		t.Fatalf("Desired state not matching actual state, expected: %+v, actual: %+v", state1, actualstate)
	}
}
