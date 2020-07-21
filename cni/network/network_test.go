package network

import (
	"fmt"
	"testing"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/common"
	acnnetwork "github.com/Azure/azure-container-networking/network"
	"github.com/Azure/azure-container-networking/telemetry"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
)

func TestAdd(t *testing.T) {
	config := &common.PluginConfig{}
	pluginName := "testplugin"

	mockNetworkManager := acnnetwork.NewMockNetworkmanager()

	plugin, _ := NewPlugin(pluginName, config)
	plugin.report = &telemetry.CNIReport{}
	plugin.nm = mockNetworkManager

	nwCfg := cni.NetworkConfig{
		Name:              "test-nwcfg",
		Type:              "azure-vnet",
		Mode:              "bridge",
		IPsToRouteViaHost: []string{"169.254.20.10"},
		Ipam: struct {
			Type          string "json:\"type\""
			Environment   string "json:\"environment,omitempty\""
			AddrSpace     string "json:\"addressSpace,omitempty\""
			Subnet        string "json:\"subnet,omitempty\""
			Address       string "json:\"ipAddress,omitempty\""
			QueryInterval string "json:\"queryInterval,omitempty\""
		}{
			Type: "azure-vnet-ipam",
		},
	}

	args := &cniSkel.CmdArgs{
		ContainerID: "test-container",
		Netns:       "test-container",
	}
	args.StdinData = nwCfg.Serialize()
	podEnv := cni.K8SPodEnvArgs{
		K8S_POD_NAME:      "test-pod",
		K8S_POD_NAMESPACE: "test-pod-namespace",
	}
	args.Args = fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", podEnv.K8S_POD_NAME, podEnv.K8S_POD_NAMESPACE)
	args.IfName = "azure0"

	plugin.Add(args)
}
