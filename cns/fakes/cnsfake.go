package fakes

import (
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/common"
)

// available IP's stack
// all IP's map

type StringStack struct {
	lock  sync.Mutex // you don't have to do this if you don't want thread safety
	items []string
}

func NewStack() *StringStack {
	return &StringStack{sync.Mutex{}, make([]string, 0)}
}

func (stack *StringStack) Push(v string) {
	stack.lock.Lock()
	defer stack.lock.Unlock()

	stack.items = append(stack.items, v)
}

func (stack *StringStack) Pop() (string, error) {
	stack.lock.Lock()
	defer stack.lock.Unlock()

	length := len(stack.items)
	if length == 0 {
		return "", errors.New("Empty Stack")
	}

	res := stack.items[length-1]
	stack.items = stack.items[:length-1]
	return res, nil
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

type IPStateManager struct {
	AvailableIPConfigState      map[string]cns.IPConfigurationStatus
	AllocatedIPConfigState      map[string]cns.IPConfigurationStatus
	PendingReleaseIPConfigState map[string]cns.IPConfigurationStatus
	AvailableIPIDStack          StringStack
	PendingAllocationIPCount    int
	sync.RWMutex
}

func NewIPStateManager() IPStateManager {
	return IPStateManager{
		AvailableIPConfigState:      make(map[string]cns.IPConfigurationStatus),
		AllocatedIPConfigState:      make(map[string]cns.IPConfigurationStatus),
		PendingReleaseIPConfigState: make(map[string]cns.IPConfigurationStatus),
		AvailableIPIDStack:          StringStack{},
		PendingAllocationIPCount:    0,
	}
}

func (ipm *IPStateManager) AddIPConfigs(ipconfigs []cns.IPConfigurationStatus) {
	ipm.Lock()
	defer ipm.Unlock()

	for i := 0; i < len(ipconfigs); i++ {
		if ipconfigs[i].State == cns.Available {
			ipm.AvailableIPConfigState[ipconfigs[i].ID] = ipconfigs[i]
			ipm.AvailableIPIDStack.Push(ipconfigs[i].ID)
		} else if ipconfigs[i].State == cns.Allocated {
			ipm.AllocatedIPConfigState[ipconfigs[i].ID] = ipconfigs[i]
		} else if ipconfigs[i].State == cns.PendingRelease {
			ipm.PendingReleaseIPConfigState[ipconfigs[i].ID] = ipconfigs[i]
		}
	}

	return
}

func (ipm *IPStateManager) ReserveIPConfig() (cns.IPConfigurationStatus, error) {
	ipm.Lock()
	defer ipm.Unlock()
	id, err := ipm.AvailableIPIDStack.Pop()
	if err != nil {
		return cns.IPConfigurationStatus{}, err
	}
	ipm.AllocatedIPConfigState[id] = ipm.AvailableIPConfigState[id]
	delete(ipm.AvailableIPConfigState, id)
	return ipm.AllocatedIPConfigState[id], nil
}

func (ipm *IPStateManager) ReleaseIPConfig(ipconfigID string) (cns.IPConfigurationStatus, error) {
	ipm.Lock()
	defer ipm.Unlock()
	ipm.AvailableIPConfigState[ipconfigID] = ipm.AllocatedIPConfigState[ipconfigID]
	ipm.AvailableIPIDStack.Push(ipconfigID)
	delete(ipm.AllocatedIPConfigState, ipconfigID)
	return ipm.AvailableIPConfigState[ipconfigID], nil
}

type HTTPServiceFake struct {
	t              *testing.T
	IPStateManager IPStateManager
	PoolMonitor    cns.IPAMPoolMonitor
}

func NewHTTPServiceFake(t *testing.T) *HTTPServiceFake {
	svc := &HTTPServiceFake{
		IPStateManager: NewIPStateManager(),
		t:              t,
	}

	return svc
}

func (fake *HTTPServiceFake) SetNumberOfAllocatedIPs(desiredAllocatedIPCount int) error {
	fake.t.Logf("[cns] Setting Allocated IPConfig count to %v", desiredAllocatedIPCount)
	currentAllocatedIPCount := len(fake.IPStateManager.AllocatedIPConfigState)
	delta := (desiredAllocatedIPCount - currentAllocatedIPCount)
	// need to free some IP's
	for i := 0; i < delta; i++ {
		_, err := fake.IPStateManager.ReserveIPConfig()
		if err != nil {
			fake.IPStateManager.PendingAllocationIPCount++
		}
	}

	// TODO decrease number of IP's
	return nil
}

func (fake *HTTPServiceFake) SendNCSnapShotPeriodically(int, chan bool) {

}

func (fake *HTTPServiceFake) SetNodeOrchestrator(*cns.SetOrchestratorTypeRequest) {

}

func (fake *HTTPServiceFake) SyncNodeStatus(string, string, string, json.RawMessage) (int, string) {
	return 0, ""
}

// TODO: change real CNS return map instead
func (fake *HTTPServiceFake) GetAvailableIPConfigs() []cns.IPConfigurationStatus {
	ipconfigs := []cns.IPConfigurationStatus{}
	for _, ipconfig := range fake.IPStateManager.AvailableIPConfigState {
		ipconfigs = append(ipconfigs, ipconfig)
	}
	return ipconfigs
}

// TODO: change real CNS return map instead
func (fake *HTTPServiceFake) GetAllocatedIPConfigs() []cns.IPConfigurationStatus {
	ipconfigs := []cns.IPConfigurationStatus{}
	for _, ipconfig := range fake.IPStateManager.AllocatedIPConfigState {
		ipconfigs = append(ipconfigs, ipconfig)
	}
	return ipconfigs
}

func (fake *HTTPServiceFake) GetPendingAllocationIPCount() int {
	return fake.IPStateManager.PendingAllocationIPCount
}

// Return union of all state maps
func (fake *HTTPServiceFake) GetPodIPConfigState() map[string]cns.IPConfigurationStatus {
	ipconfigs := make(map[string]cns.IPConfigurationStatus)
	for key, val := range fake.IPStateManager.AllocatedIPConfigState {
		ipconfigs[key] = val
	}
	for key, val := range fake.IPStateManager.AvailableIPConfigState {
		ipconfigs[key] = val
	}
	for key, val := range fake.IPStateManager.PendingReleaseIPConfigState {
		ipconfigs[key] = val
	}
	return ipconfigs
}

func (fake *HTTPServiceFake) AllocateTestIPConfigsToPendingPods() error {
	for fake.IPStateManager.PendingAllocationIPCount > 0 {
		_, err := fake.IPStateManager.ReserveIPConfig()
		if err != nil {
			return err
		}

		fake.IPStateManager.PendingAllocationIPCount--
	}

	return nil
}

// TODO: Populate on scale down
func (fake *HTTPServiceFake) MarkIPsAsPending(numberToMark int) (map[string]cns.SecondaryIPConfig, error) {
	return make(map[string]cns.SecondaryIPConfig), nil
}

func (fake *HTTPServiceFake) GetOption(string) interface{} {
	return nil
}

func (fake *HTTPServiceFake) SetOption(string, interface{}) {

}

func (fake *HTTPServiceFake) Start(*common.ServiceConfig) error {
	return nil
}

func (fake *HTTPServiceFake) Stop() {

}
