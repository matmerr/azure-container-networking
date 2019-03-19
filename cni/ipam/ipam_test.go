// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipam

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/common"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
)

var plugin *ipamPlugin
var args cniSkel.CmdArgs

var ipamQueryUrl = "localhost:42424"
var ipamQueryResponse = "" +
	"<Interfaces>" +
	"	<Interface MacAddress=\"*\" IsPrimary=\"true\">" +
	"		<IPSubnet Prefix=\"10.0.0.0/16\">" +
	"			<IPAddress Address=\"10.0.0.4\" IsPrimary=\"true\"/>" +
	"			<IPAddress Address=\"10.0.0.5\" IsPrimary=\"false\"/>" +
	"			<IPAddress Address=\"10.0.0.6\" IsPrimary=\"false\"/>" +
	"		</IPSubnet>" +
	"	</Interface>" +
	"</Interfaces>"

var localAsId string
var poolId1 string
var address1 string

// Wraps the test run with plugin setup and teardown.
func TestMain(m *testing.M) {
	var config common.PluginConfig

	// Create a fake local agent to handle requests from IPAM plugin.
	u, _ := url.Parse("tcp://" + ipamQueryUrl)
	testAgent, err := common.NewListener(u)
	if err != nil {
		fmt.Printf("Failed to create agent, err:%v.\n", err)
		return
	}
	testAgent.AddHandler("/", handleIpamQuery)

	err = testAgent.Start(make(chan error, 1))
	if err != nil {
		fmt.Printf("Failed to start agent, err:%v.\n", err)
		return
	}

	// Create the plugin.
	plugin, err = NewPlugin(&config)
	if err != nil {
		fmt.Printf("Failed to create IPAM plugin, err:%v.\n", err)
		return
	}

	// Configure test mode.
	plugin.SetOption(common.OptEnvironment, common.OptEnvironmentAzure)
	plugin.SetOption(common.OptAPIServerURL, "null")
	plugin.SetOption(common.OptIpamQueryUrl, "http://"+ipamQueryUrl)

	// Start the plugin.
	err = plugin.Start(&config)
	if err != nil {
		fmt.Printf("Failed to start IPAM plugin, err:%v.\n", err)
		return
	}

	// Run tests.
	exitCode := m.Run()

	// Cleanup.
	plugin.Stop()
	testAgent.Stop()

	os.Exit(exitCode)
}

// Handles queries from IPAM source.
func handleIpamQuery(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(ipamQueryResponse))
}

//
// CNI IPAM API compliance tests
// https://github.com/containernetworking/cni/blob/master/SPEC.md
//

func setValidCNIArgs() {
	args = cniSkel.CmdArgs{}
	nwCfg := cni.NetworkConfig{}
	args.StdinData, _ = json.Marshal(nwCfg)
	return
}

func TestAddSuccess(t *testing.T) {
	setValidCNIArgs()
	plugin.Add(&args)
}

func TestDelSuccess(t *testing.T) {
	setValidCNIArgs()
	plugin.Delete(&args)
}

func TestGetSuccess(t *testing.T) {
	setValidCNIArgs()
	plugin.Get(&args)
}

func TestUpdateSuccess(t *testing.T) {
	setValidCNIArgs()
	plugin.Update(&args)
}
