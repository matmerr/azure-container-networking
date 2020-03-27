// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipam

import (
	"context"
	"reflect"
	"testing"

	"github.com/Azure/azure-container-networking/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestIPv6Ipam(t *testing.T) {
	options := make(map[string]interface{})
	options[common.OptEnvironment] = common.OptEnvironmentIPv6Ipam

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

	client := testclient.NewSimpleClientset()
	client.CoreV1().Nodes().Create(context.TODO(), testnode, metav1.CreateOptions{})
	node, _ := client.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})

	testInterfaces, err := retrieveKubernetesPodIPs(node, "::/127")
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
							{Address: "ace:cab:deca:deed::1", IsPrimary: false},
						},
					},
				},
			},
		},
	}

	equal := reflect.DeepEqual(testInterfaces, correctInterfaces)

	if !equal {
		t.Fatalf("Network interface mismatch, expected: %+v actual: %+v", correctInterfaces, testInterfaces)
	}
}
