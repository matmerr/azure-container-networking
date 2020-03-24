// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipam

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Azure/azure-container-networking/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesIpam(t *testing.T) {

	options := make(map[string]interface{})
	options[common.OptEnvironment] = common.OptEnvironmentIPv6Ipam

	client := testclient.NewSimpleClientset()
	nodeName := "TestNode"

	testnode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: v1.NodeSpec{
			PodCIDR:  "10.0.0.1/24",
			PodCIDRs: []string{"10.0.0.1/24", "ace:cab:deca:deed::/64"},
		},
	}

	client.CoreV1().Nodes().Create(testnode)

	node, _ := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})

	testInterfaces, err := carveAddresses(node, "::/127")

	if err != nil {
		t.Fatalf("Failed to carve addresses: %+v", err)
	}

	correctInterfaces := &NetworkInterfaces{
		Interfaces: []Interface{
			{
				IsPrimary: true,
				IPSubnets: []IPSubnet{
					{
						Prefix: "ace:cab:deca:deed::/127",
						IPAddresses: []IPAddress{
							{Address: "ace:cab:deca:deed::", IsPrimary: true},
							{Address: "ace:cab:deca:deed::1", IsPrimary: false},
						},
					},
				},
			},
		},
	}

	isequal := reflect.DeepEqual(testInterfaces, correctInterfaces)
	fmt.Println(isequal)

	/*
		err := ipam.RefreshKubernetesIpam(client, nodeName)
		if err != nil {
			t.Fatalf("Failed to retrieve node spec with error: %+v", err)
		}

		filepath := "/tmp/nodespec.json"

		jsonFile, _ := os.Open(filepath)
		byteValue, err := ioutil.ReadAll(jsonFile)

		if err != nil {
			t.Fatalf("Failed to load saved node spec with error: %+v", err)
		}

		validateNodeSpec := v1.NodeSpec{}
		json.Unmarshal(byteValue, &validateNodeSpec)

		if testnode.Spec.PodCIDR != testnode.Spec.PodCIDR {
			t.Fatalf("Node validation failed, expected: %+v, actual: %+v", testnode.Spec.PodCIDR, testnode.Spec.PodCIDR)
		}

		if !reflect.DeepEqual(testnode.Spec.PodCIDRs, validateNodeSpec.PodCIDRs) {
			t.Fatalf("Node validation failed, expected: %+v, actual: %+v", testnode.Spec.PodCIDRs, validateNodeSpec.PodCIDRs)
		}
	*/
}
