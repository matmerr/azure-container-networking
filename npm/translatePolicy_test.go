package npm

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/Azure/azure-container-networking/npm/iptm"
	"github.com/Azure/azure-container-networking/npm/util"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestGetDefaultDropEntries(t *testing.T) {
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

	iptIngressEntries := getDefaultDropEntries(ns, targetSelector, true, false)

	expectedIptIngressEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureTargetSetsChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("context:dev"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testNotIn:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesDrop,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"DROP-ALL-TO-context:dev-AND-!testNotIn:frontend",
			},
		},
	}

	if !reflect.DeepEqual(iptIngressEntries, expectedIptIngressEntries) {
		t.Errorf("TestGetDefaultDropEntries failed @ iptEntries comparison")
		marshalledIptEntries, _ := json.Marshal(iptIngressEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptIngressEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}

	iptEgressEntries := getDefaultDropEntries(ns, targetSelector, false, true)

	expectedIptEgressEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureTargetSetsChain,
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
				util.IptablesJumpFlag,
				util.IptablesDrop,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"DROP-ALL-FROM-context:dev-AND-!testNotIn:frontend",
			},
		},
	}

	if !reflect.DeepEqual(iptEgressEntries, expectedIptEgressEntries) {
		t.Errorf("TestGetDefaultDropEntries failed @ iptEntries comparison")
		marshalledIptEntries, _ := json.Marshal(iptEgressEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEgressEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}

	iptIngressEgressEntries := getDefaultDropEntries(ns, targetSelector, true, true)

	expectedIptIngressEgressEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureTargetSetsChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("context:dev"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testNotIn:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesDrop,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"DROP-ALL-TO-context:dev-AND-!testNotIn:frontend",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureTargetSetsChain,
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
				util.IptablesJumpFlag,
				util.IptablesDrop,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"DROP-ALL-FROM-context:dev-AND-!testNotIn:frontend",
			},
		},
	}

	if !reflect.DeepEqual(iptIngressEgressEntries, expectedIptIngressEgressEntries) {
		t.Errorf("TestGetDefaultDropEntries failed @ iptEntries comparison")
		marshalledIptEntries, _ := json.Marshal(iptIngressEgressEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptIngressEgressEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestTranslatePolicy(t *testing.T) {

}

func TestAllowPrecedenceOverDeny(t *testing.T) {
	targetSelector := metav1.LabelSelector{}
	targetSelectorA := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "test",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			metav1.LabelSelectorRequirement{
				Key:      "testIn",
				Operator: metav1.LabelSelectorOpIn,
				Values: []string{
					"pod-A",
				},
			},
		},
	}
	denyAllPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-deny",
			Namespace: "default",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: targetSelector,
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{},
		},
	}
	allowToPodPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-A",
			Namespace: "default",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: targetSelectorA,
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				networkingv1.NetworkPolicyIngressRule{
					From: []networkingv1.NetworkPolicyPeer{
						networkingv1.NetworkPolicyPeer{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "test",
								},
								MatchExpressions: []metav1.LabelSelectorRequirement{
									metav1.LabelSelectorRequirement{
										Key:      "testIn",
										Operator: metav1.LabelSelectorOpIn,
										Values: []string{
											"pod-B",
										},
									},
								},
							},
						},
						networkingv1.NetworkPolicyPeer{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "test",
								},
								MatchExpressions: []metav1.LabelSelectorRequirement{
									metav1.LabelSelectorRequirement{
										Key:      "testIn",
										Operator: metav1.LabelSelectorOpIn,
										Values: []string{
											"pod-C",
										},
									},
								},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				networkingv1.NetworkPolicyEgressRule{
					To: []networkingv1.NetworkPolicyPeer{
						networkingv1.NetworkPolicyPeer{
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
				},
			},
		},
	}

	sets, lists, iptEntries := translatePolicy(denyAllPolicy)
	expectedSets := []string{
		"ns-default",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ k8s-example-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ k8s-example-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	sets, lists, finalIptEntries := translatePolicy(allowToPodPolicy)
	expectedSets = []string{
		"app:test",
		"testIn:pod-A",
		"testIn:pod-B",
		"testIn:pod-C",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ k8s-example-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists = []string{
		"all-namespaces",
	}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ k8s-example-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	iptEntries = append(iptEntries, finalIptEntries...)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureTargetSetsChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-default"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesDrop,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"DROP-ALL-TO-ns-default",
			},
		},
	}
	nonKubeSystemEntries2 := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:test"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testIn:pod-A"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:test-AND-testIn:pod-A-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:test"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testIn:pod-B"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:test"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testIn:pod-A"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-app:test-AND-testIn:pod-B-TO-app:test-AND-testIn:pod-A",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:test"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testIn:pod-C"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:test"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testIn:pod-A"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-app:test-AND-testIn:pod-C-TO-app:test-AND-testIn:pod-A",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:test"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testIn:pod-A"),
				util.IptablesSrcFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureEgressToChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-FROM-app:test-AND-testIn:pod-A-TO-JUMP-TO-" +
					util.IptablesAzureEgressToChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressToChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:test"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("testIn:pod-A"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("all-namespaces"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-app:test-AND-testIn:pod-A-TO-all-namespaces",
			},
		},
	}
	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(expectedIptEntries, getAllowKubeSystemEntries("default", targetSelector)...)
	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getAllowKubeSystemEntries("default", targetSelectorA)...)
	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries2...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("default", targetSelectorA, true, true)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("TestAllowPrecedenceOverDeny failed @ k8s-example-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func readPolicyYaml(policyYaml string) (*networkingv1.NetworkPolicy, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	b, err := ioutil.ReadFile(policyYaml)
	if err != nil {
		return nil, err
	}
	obj, _, err := decode([]byte(b), nil, nil)
	if err != nil {
		return nil, err
	}
	return obj.(*networkingv1.NetworkPolicy), nil
}

func TestDenyAll(t *testing.T) {

	denyAllPolicy, err := readPolicyYaml("testpolicies/deny-all-policy.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(denyAllPolicy)

	expectedSets := []string{"ns-testnamespace"}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ deny-all-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ deny-all-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", denyAllPolicy.Spec.PodSelector)...,
	)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", denyAllPolicy.Spec.PodSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ deny-all-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowBackendToFrontend(t *testing.T) {

	allowBackendToFrontendPolicy, err := readPolicyYaml("testpolicies/allow-backend-to-frontend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowBackendToFrontendPolicy)

	expectedSets := []string{
		"app:backend",
		"app:frontend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-app:backend-TO-app:frontend-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-app:backend-TO-app:frontend-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", allowBackendToFrontendPolicy.Spec.PodSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:backend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:backend-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:backend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-app:frontend-TO-app:backend",
			},
		},
	}
	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", allowBackendToFrontendPolicy.Spec.PodSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-app:frontend-TO-app:backend-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}

}

func TestAllowAllToAppFrontend(t *testing.T) {

	allowToFrontendPolicy, err := readPolicyYaml("testpolicies/allow-all-to-app-frontend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowToFrontendPolicy)

	expectedSets := []string{
		"app:frontend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-all-TO-app:frontend-FROM-all-namespaces-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{
		util.KubeAllNamespacesFlag,
	}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-all-TO-app:frontend-FROM-all-namespaces-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", allowToFrontendPolicy.Spec.PodSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName(util.KubeAllNamespacesFlag),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:frontend-FROM-all-namespaces",
			},
		},
	}
	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", allowToFrontendPolicy.Spec.PodSelector, false, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-all-TO-app:frontend-FROM-all-namespaces-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}
func TestDenyAllToAppFrontend(t *testing.T) {

	targetSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "frontend",
		},
	}

	denyAllToFrontendPolicy, err := readPolicyYaml("testpolicies/deny-all-to-app-frontend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(denyAllToFrontendPolicy)

	expectedSets := []string{
		"app:frontend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-none-TO-app:frontend-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-none-TO-app:frontend-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", targetSelector)...,
	)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", targetSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-none-TO-app:frontend-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestNamespaceToFrontend(t *testing.T) {

	targetSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "frontend",
		},
	}

	allowNsTestNamespaceToFrontendPolicy, err := readPolicyYaml("testpolicies/allow-ns-test-namespace-to-frontend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowNsTestNamespaceToFrontendPolicy)

	expectedSets := []string{
		"app:frontend",
		"ns-testnamespace",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-ns-testnamespace-TO-app:frontend-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-ns-testnamespace-TO-app:frontend-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", targetSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:frontend-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-testnamespace"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ns-testnamespace-TO-app:frontend",
			},
		},
	}
	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", targetSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-ns-testnamespace-TO-app:frontend-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowAllNamespacesToAppFrontend(t *testing.T) {

	targetSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "frontend",
		},
	}

	allowAllNsToFrontendPolicy, err := readPolicyYaml("testpolicies/allow-all-ns-to-frontend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowAllNsToFrontendPolicy)
	expectedSets := []string{
		"app:frontend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-all-namespaces-TO-app:frontend-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{
		util.KubeAllNamespacesFlag,
	}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-all-namespaces-TO-app:frontend-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", targetSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:frontend-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName(util.KubeAllNamespacesFlag),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-all-namespaces-TO-app:frontend",
			},
		},
	}
	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", targetSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-all-namespaces-TO-app:frontend-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowNamespaceDevToAppFrontend(t *testing.T) {

	allowNsDevToFrontendPolicy, err := readPolicyYaml("testpolicies/allow-ns-dev-to-app-frontend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowNsDevToFrontendPolicy)

	expectedSets := []string{
		"app:frontend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-ns-namespace:dev-AND-!ns-namespace:test0-AND-!ns-namespace:test1-TO-app:frontend-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{
		"ns-namespace:dev",
		"ns-namespace:test0",
		"ns-namespace:test1",
	}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-ns-namespace:dev-AND-!ns-namespace:test0-AND-!ns-namespace:test1-TO-app:frontend-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", allowNsDevToFrontendPolicy.Spec.PodSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:frontend-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-namespace:dev"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-namespace:test0"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-namespace:test1"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ns-namespace:dev-AND-ns-!namespace:test0-AND-ns-!namespace:test1-TO-app:frontend",
			},
		},
	}

	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", allowNsDevToFrontendPolicy.Spec.PodSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-ns-namespace:dev-AND-!ns-namespace:test0-AND-!ns-namespace:test1-TO-app:frontend-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowAllToK0AndK1AndAppFrontend(t *testing.T) {

	allowAllToFrontendPolicy, err := readPolicyYaml("testpolicies/test-allow-all-to-k0-and-k1-and-app-frontend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowAllToFrontendPolicy)

	expectedSets := []string{
		"app:frontend",
		"k0",
		"k1:v0",
		"k1:v1",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ AllOW-ALL-TO-k0-AND-k1:v0-AND-k1:v1-AND-app:frontend-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{util.KubeAllNamespacesFlag}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ AllOW-ALL-TO-k0-AND-k1:v0-AND-k1:v1-AND-app:frontend-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", allowAllToFrontendPolicy.Spec.PodSelector)...,
	)
	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("k0"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("k1:v0"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("k1:v1"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:frontend-AND-!k0-AND-k1:v0-AND-k1:v1-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName(util.KubeAllNamespacesFlag),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesNotFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("k0"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("k1:v0"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("k1:v1"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-all-namespaces-TO-app:frontend-AND-!k0-AND-k1:v0-AND-k1:v1",
			},
		},
	}

	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", allowAllToFrontendPolicy.Spec.PodSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ AllOW-all-TO-k0-AND-k1:v0-AND-k1:v1-AND-app:frontend-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowNsDevAndAppBackendToAppFrontend(t *testing.T) {

	allowNsDevAndBackendToFrontendPolicy, err := readPolicyYaml("testpolicies/allow-ns-dev-and-app-backend-to-app-frontend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	util.IsNewNwPolicyVerFlag = true
	sets, lists, iptEntries := translatePolicy(allowNsDevAndBackendToFrontendPolicy)

	expectedSets := []string{
		"app:frontend",
		"app:backend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-ns-ns:dev-AND-app:backend-TO-app:frontend sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{
		"ns-ns:dev",
	}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-ns-ns:dev-AND-app:backend-TO-app:frontend lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", allowNsDevAndBackendToFrontendPolicy.Spec.PodSelector)...,
	)
	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:frontend-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-ns:dev"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:backend"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ns-ns:dev-AND-app:backend-TO-app:frontend",
			},
		},
	}

	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", allowNsDevAndBackendToFrontendPolicy.Spec.PodSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-ns-ns:dev-AND-app:backend-TO-app:frontend policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowInternalAndExternal(t *testing.T) {

	allowInternalAndExternalPolicy, err := readPolicyYaml("testpolicies/allow-internal-and-external.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowInternalAndExternalPolicy)

	expectedSets := []string{
		"app:backdoor",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-ALL-TO-app:backdoor-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-ALL-TO-app:backdoor-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", allowInternalAndExternalPolicy.Spec.PodSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:backdoor"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:backdoor-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:backdoor"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:backdoor",
			},
		},
	}

	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("dangerous", allowInternalAndExternalPolicy.Spec.PodSelector, false, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-ALL-TO-app:backdoor-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowBackendToFrontendPort8000(t *testing.T) {

	allowBackendToFrontendPort8000Policy, err := readPolicyYaml("testpolicies/allow-app-backend-to-app-frontend-port-8000.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowBackendToFrontendPort8000Policy)

	expectedSets := []string{
		"app:frontend",
		"app:backend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-app:backend-TO-app:frontend-port-8000-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-app:backend-TO-app:frontend-port-8000-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", allowBackendToFrontendPort8000Policy.Spec.PodSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesDstPortFlag,
				"8000",
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-PORT-8000-OF-app:frontend-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:backend"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-app:backend-TO-app:frontend",
			},
		},
	}

	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("dangerous", allowBackendToFrontendPort8000Policy.Spec.PodSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-ALL-TO-app:backdoor-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowMultipleLabelsToMultipleLabels(t *testing.T) {

	allowCniOrCnsToK8sPolicy, err := readPolicyYaml("testpolicies/allow-multiple-labels-to-multiple-labels.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowCniOrCnsToK8sPolicy)

	expectedSets := []string{
		"app:k8s",
		"team:aks",
		"program:cni",
		"team:acn",
		"binary:cns",
		"group:container",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-program:cni-AND-team:acn-OR-binary:cns-AND-group:container-TO-app:k8s-AND-team:aks-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-program:cni-AND-team:acn-OR-binary:cns-AND-group:container-TO-app:k8s-AND-team:aks-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("acn", allowCniOrCnsToK8sPolicy.Spec.PodSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:k8s"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("team:aks"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-app:k8s-AND-team:aks-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("program:cni"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("team:acn"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:k8s"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("team:aks"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-program:cni-AND-team:acn-TO-app:k8s-AND-team:aks",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("binary:cns"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("group:container"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:k8s"),
				util.IptablesDstFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("team:aks"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-binary:cns-AND-group:container-TO-app:k8s-AND-team:aks",
			},
		},
	}

	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("acn", allowCniOrCnsToK8sPolicy.Spec.PodSelector, true, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-program:cni-AND-team:acn-OR-binary:cns-AND-group:container-TO-app:k8s-AND-team:aks-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestDenyAllFromAppBackend(t *testing.T) {

	denyAllFromBackendPolicy, err := readPolicyYaml("testpolicies/deny-all-from-app-backend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(denyAllFromBackendPolicy)

	expectedSets := []string{
		"app:backend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-none-FROM-app:backend-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-none-FROM-app:backend-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", denyAllFromBackendPolicy.Spec.PodSelector)...,
	)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", denyAllFromBackendPolicy.Spec.PodSelector, false, true)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-none-FROM-app:backend-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowAllFromAppBackend(t *testing.T) {

	allowAllEgress, err := readPolicyYaml("testpolicies/allow-all-from-app-backend.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowAllEgress)

	expectedSets := []string{
		"app:backend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-all-FROM-app:backend-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{
		util.KubeAllNamespacesFlag,
	}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-all-FROM-app:backend-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", allowAllEgress.Spec.PodSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:backend"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName(util.KubeAllNamespacesFlag),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-FROM-app:backend-TO-" +
					util.KubeAllNamespacesFlag,
			},
		},
	}
	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	// has egress, but empty map means allow all
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", allowAllEgress.Spec.PodSelector, false, false)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-all-FROM-app:backend-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestDenyAllFromNsUnsafe(t *testing.T) {

	denyAllFromNsUnsafePolicy, err := readPolicyYaml("testpolicies/deny-all-from-ns-unsafe.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(denyAllFromNsUnsafePolicy)

	expectedSets := []string{
		"ns-unsafe",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-none-FROM-ns-unsafe-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}
	expectedLists := []string{}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-none-FROM-app:backend-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("unsafe", denyAllFromNsUnsafePolicy.Spec.PodSelector)...,
	)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("unsafe", denyAllFromNsUnsafePolicy.Spec.PodSelector, false, true)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-none-FROM-app:backend-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func TestAllowAppFrontendToTCPPort53UDPPort53Policy(t *testing.T) {

	allowFrontendToTCPPort53UDPPort53Policy, err := readPolicyYaml("testpolicies/allow-app-frontend-tcp-port-or-udp-port-53.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(allowFrontendToTCPPort53UDPPort53Policy)

	expectedSets := []string{
		"app:frontend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ ALLOW-ALL-FROM-app:frontend-TCP-PORT-53-OR-UDP-PORT-53-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{
		util.KubeAllNamespacesFlag,
	}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ ALLOW-ALL-FROM-app:frontend-TCP-PORT-53-OR-UDP-PORT-53-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", allowFrontendToTCPPort53UDPPort53Policy.Spec.PodSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressPortChain,
			Specs: []string{
				util.IptablesProtFlag,
				"TCP",
				util.IptablesDstPortFlag,
				"53",
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesSrcFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-FROM-TCP-PORT-53-OF-app:frontend",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressPortChain,
			Specs: []string{
				util.IptablesProtFlag,
				"UDP",
				util.IptablesDstPortFlag,
				"53",
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesSrcFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-FROM-UDP-PORT-53-OF-app:frontend",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressPortChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesSrcFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureEgressToChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-FROM-app:frontend-TO-JUMP-TO-" +
					util.IptablesAzureEgressToChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressToChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("app:frontend"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName(util.KubeAllNamespacesFlag),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-app:frontend-TO-" +
					util.KubeAllNamespacesFlag,
			},
		},
	}
	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", allowFrontendToTCPPort53UDPPort53Policy.Spec.PodSelector, false, true)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ ALLOW-ALL-FROM-app:frontend-TCP-PORT-53-OR-UDP-PORT-53-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}

func Test17(t *testing.T) {

	k8sExamplePolicy, err := readPolicyYaml("testpolicies/complex-policy.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sets, lists, iptEntries := translatePolicy(k8sExamplePolicy)

	expectedSets := []string{
		"role:db",
		"role:frontend",
	}
	if !reflect.DeepEqual(sets, expectedSets) {
		t.Errorf("translatedPolicy failed @ k8s-example-policy sets comparison")
		t.Errorf("sets: %v", sets)
		t.Errorf("expectedSets: %v", expectedSets)
	}

	expectedLists := []string{
		"ns-project:myproject",
	}
	if !reflect.DeepEqual(lists, expectedLists) {
		t.Errorf("translatedPolicy failed @ k8s-example-policy lists comparison")
		t.Errorf("lists: %v", lists)
		t.Errorf("expectedLists: %v", expectedLists)
	}

	expectedIptEntries := []*iptm.IptEntry{}
	expectedIptEntries = append(
		expectedIptEntries,
		getAllowKubeSystemEntries("testnamespace", k8sExamplePolicy.Spec.PodSelector)...,
	)

	nonKubeSystemEntries := []*iptm.IptEntry{
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressPortChain,
			Specs: []string{
				util.IptablesProtFlag,
				"TCP",
				util.IptablesDstPortFlag,
				"6379",
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("role:db"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureIngressFromChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-TCP-PORT-6379-OF-role:db-TO-JUMP-TO-" +
					util.IptablesAzureIngressFromChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesSFlag,
				"172.17.0.0/16",
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("role:db"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-172.17.0.0/16-TO-role:db",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesSFlag,
				"172.17.1.0/24",
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("role:db"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesDrop,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"DROP-172.17.1.0/24-TO-role:db",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("ns-project:myproject"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("role:db"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ns-project:myproject-TO-role:db",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureIngressFromChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("role:frontend"),
				util.IptablesSrcFlag,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("role:db"),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-role:frontend-TO-role:db",
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressPortChain,
			Specs: []string{
				util.IptablesProtFlag,
				"TCP",
				util.IptablesDstPortFlag,
				"5978",
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("role:db"),
				util.IptablesSrcFlag,
				util.IptablesJumpFlag,
				util.IptablesAzureEgressToChain,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-FROM-TCP-PORT-5978-OF-role:db-TO-JUMP-TO-" +
					util.IptablesAzureEgressToChain,
			},
		},
		&iptm.IptEntry{
			Chain: util.IptablesAzureEgressToChain,
			Specs: []string{
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName("role:db"),
				util.IptablesSrcFlag,
				util.IptablesDFlag,
				"10.0.0.0/24",
				util.IptablesJumpFlag,
				util.IptablesAccept,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-10.0.0.0/24-FROM-role:db",
			},
		},
	}
	expectedIptEntries = append(expectedIptEntries, nonKubeSystemEntries...)
	expectedIptEntries = append(expectedIptEntries, getDefaultDropEntries("testnamespace", k8sExamplePolicy.Spec.PodSelector, true, true)...)
	if !reflect.DeepEqual(iptEntries, expectedIptEntries) {
		t.Errorf("translatedPolicy failed @ k8s-example-policy policy comparison")
		marshalledIptEntries, _ := json.Marshal(iptEntries)
		marshalledExpectedIptEntries, _ := json.Marshal(expectedIptEntries)
		t.Errorf("iptEntries: %s", marshalledIptEntries)
		t.Errorf("expectedIptEntries: %s", marshalledExpectedIptEntries)
	}
}
