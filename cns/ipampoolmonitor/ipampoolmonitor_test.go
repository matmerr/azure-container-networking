package ipampoolmonitor

import (
	"testing"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/fakes"
	"github.com/Azure/azure-container-networking/cns/logger"
)

func TestInterfaces(t *testing.T) {
	logger.InitLogger("testlogs", 0, 0, "./")

	scalarUnits := cns.ScalarUnits{
		BatchSize:               10,
		IPConfigCount:           10,
		RequestThresholdPercent: 30,
		ReleaseThresholdPercent: 150,
	}

	initialAvailableIPConfigCount := 10

	fakecns := fakes.NewHTTPServiceFake()
	fakerc := fakes.NewRequestControllerFake(fakecns, scalarUnits, initialAvailableIPConfigCount)
	poolmonitor := NewCNSIPAMPoolMonitor(fakecns, fakerc)
	fakecns.PoolMonitor = poolmonitor

	poolmonitor.UpdatePoolLimitsTransacted(scalarUnits)

	t.Logf("Minimum free IPs to request: %v", poolmonitor.MinimumFreeIps)
	t.Logf("Maximum free IPs to release: %v", poolmonitor.MaximumFreeIps)

	err := fakecns.SetNumberOfAllocatedIPs(8)

	poolmonitor.Reconcile()

	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err %v", err)
	}

	poolmonitor.Reconcile()

	err = fakecns.SetNumberOfAllocatedIPs(9)
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err %v", err)
	}

	poolmonitor.Reconcile()

}
