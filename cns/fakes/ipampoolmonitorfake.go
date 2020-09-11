package fakes

import (
	"context"

	"github.com/Azure/azure-container-networking/cns"
)

type IPAMPoolMonitorFake struct{}

func NewIPAMPoolMonitorFake() *IPAMPoolMonitorFake {
	return &IPAMPoolMonitorFake{}
}

func (ipm *IPAMPoolMonitorFake) Start(ctx context.Context, poolMonitorRefreshMilliseconds int) error {
	return nil
}

func (ipm *IPAMPoolMonitorFake) UpdatePoolMonitor(scalarUnits cns.ScalarUnits) error {
	return nil
}

func (ipm *IPAMPoolMonitorFake) Reconcile() error {
	return nil
}
