package npm

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Azure/azure-container-networking/npm/iptm"
	"github.com/Azure/azure-container-networking/npm/util"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestTranslateEgress(t *testing.T) {
	ns := "testnamespace"

	targetSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"context": "dev",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			metav1.LabelSelectorRequirement{
				Key:      "testNotIn",
				Operator: metav1.LabelSelectorOpNotIn,
				Values: []string{
					"frontend",
				},
			},
		},
	}

	tcp := v1.ProtocolTCP
	port6783 := intstr.FromInt(6783)
	egressPodSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "db",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			metav1.LabelSelectorRequirement{
				Key:      "testIn",
				Operator: metav1.LabelSelectorOpIn,
				Values: []string{
					"frontend",
				},
			},
		},
	}
	egressNamespaceSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"ns": "dev",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			metav1.LabelSelectorRequirement{
				Key:      "testIn",
				Operator: metav1.LabelSelectorOpIn,
				Values: []string{
					"frontendns",
				},
			},
		},
	}

	compositeNetworkPolicyPeer := networkingv1.NetworkPolicyPeer{
		PodSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"region": "northpole",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "k",
					Operator: metav1.LabelSelectorOpDoesNotExist,
				},
			},
		},
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"planet": "earth",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "keyExists",
					Operator: metav1.LabelSelectorOpExists,
				},
			},
		},
	}

	rules := []networkingv1.NetworkPolicyEgressRule{
		networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{
				networkingv1.NetworkPolicyPort{
					Protocol: &tcp,
					Port:     &port6783,
				},
			},
			To: []networkingv1.NetworkPolicyPeer{
				networkingv1.NetworkPolicyPeer{
					PodSelector: egressPodSelector,
				},
				networkingv1.NetworkPolicyPeer{
					NamespaceSelector: egressNamespaceSelector,
				},
				compositeNetworkPolicyPeer,
			},
		},
	}

	util.IsNewNwPolicyVerFlag = true
	sets, lists, iptEntries := translateEgress(ns, targetSelector, rules)
	expectedSets := []string{
		"context:dev",
		"testNotIn:frontend",
		"app:db",
		"testIn:frontend",
		"region:northpole",
		"k",
	}

	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedEgress failed @ sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{
		"ns-ns:dev",
		"ns-testIn:frontendns",
		"ns-planet:earth",
		"ns-keyExists",
	}

	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedEgress failed @ lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressPortChain,
			Specs: []string{
				util.IptablesProtFlag,
				string(v1.ProtocolTCP),
				util.IptablesDstPortFlag,
				"6783",
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("context:dev"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testNotIn:frontend"),
				util.IptablesSrcFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureEgressToChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-FROM-TCP-PORT-6783-OF-context:dev-AND-!testNotIn:frontend-TO-JUMP-TO-" +
					util.IptablesAzureEgressToChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressToChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("context:dev"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testNotIn:frontend"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:db"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testIn:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-context:dev-AND-!testNotIn:frontend-TO-app:db-AND-testIn:frontend",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressToChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("context:dev"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testNotIn:frontend"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-ns:dev"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-testIn:frontendns"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-context:dev-AND-!testNotIn:frontend-TO-ns-ns:dev-AND-ns-testIn:frontendns",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressToChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("context:dev"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testNotIn:frontend"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-planet:earth"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-keyExists"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("region:northpole"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("k"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-context:dev-AND-!testNotIn:frontend-TO-ns-planet:earth-AND-ns-keyExists-AND-region:northpole-AND-!k",
			},
		},
	}

	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedEgress failed @ composite egress rule comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}
