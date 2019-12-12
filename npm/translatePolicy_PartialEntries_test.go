package npm

import (
	"reflect"
	"testing"

	"github.com/Azure/azure-container-networking/npm/util"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestCraftPartialIptEntrySpecFromPort(t *testing.T) {
	portRule := networkingv1.NetworkPolicyPort{}

	iptEntrySpec := craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)
	expectedIptEntrySpec := []string{}

	if !reflect.DeepEqual(iptEntrySpec, expectedIptEntrySpec) {
		t.Errorf("TestCraftPartialIptEntrySpecFromPort failed @ empty iptEntrySpec comparison")
		t.Errorf("iptEntrySpec:\n%v", iptEntrySpec)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedIptEntrySpec)
	}

	tcp := v1.ProtocolTCP
	portRule = networkingv1.NetworkPolicyPort{
		Protocol: &tcp,
	}

	iptEntrySpec = craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)
	expectedIptEntrySpec = []string{
		util.IptablesProtFlag,
		"TCP",
	}

	if !reflect.DeepEqual(iptEntrySpec, expectedIptEntrySpec) {
		t.Errorf("TestCraftPartialIptEntrySpecFromPort failed @ tcp iptEntrySpec comparison")
		t.Errorf("iptEntrySpec:\n%v", iptEntrySpec)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedIptEntrySpec)
	}

	port8000 := intstr.FromInt(8000)
	portRule = networkingv1.NetworkPolicyPort{
		Port: &port8000,
	}

	iptEntrySpec = craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)
	expectedIptEntrySpec = []string{
		util.IptablesDstPortFlag,
		"8000",
	}

	if !reflect.DeepEqual(iptEntrySpec, expectedIptEntrySpec) {
		t.Errorf("TestCraftPartialIptEntrySpecFromPort failed @ port 8000 iptEntrySpec comparison")
		t.Errorf("iptEntrySpec:\n%v", iptEntrySpec)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedIptEntrySpec)
	}

	portRule = networkingv1.NetworkPolicyPort{
		Protocol: &tcp,
		Port:     &port8000,
	}

	iptEntrySpec = craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)
	expectedIptEntrySpec = []string{
		util.IptablesProtFlag,
		"TCP",
		util.IptablesDstPortFlag,
		"8000",
	}

	if !reflect.DeepEqual(iptEntrySpec, expectedIptEntrySpec) {
		t.Errorf("TestCraftPartialIptEntrySpecFromPort failed @ tcp port 8000 iptEntrySpec comparison")
		t.Errorf("iptEntrySpec:\n%v", iptEntrySpec)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedIptEntrySpec)
	}
}

func TestCraftPartialIptablesCommentFromPort(t *testing.T) {
	portRule := networkingv1.NetworkPolicyPort{}

	comment := craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)
	expectedComment := ""

	if !reflect.DeepEqual(comment, expectedComment) {
		t.Errorf("TestCraftPartialIptablesCommentFromPort failed @ empty comment comparison")
		t.Errorf("comment:\n%v", comment)
		t.Errorf("expectedComment:\n%v", expectedComment)
	}

	tcp := v1.ProtocolTCP
	portRule = networkingv1.NetworkPolicyPort{
		Protocol: &tcp,
	}

	comment = craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)
	expectedComment = "TCP-OF-"

	if !reflect.DeepEqual(comment, expectedComment) {
		t.Errorf("TestCraftPartialIptablesCommentFromPort failed @ tcp comment comparison")
		t.Errorf("comment:\n%v", comment)
		t.Errorf("expectedComment:\n%v", expectedComment)
	}

	port8000 := intstr.FromInt(8000)
	portRule = networkingv1.NetworkPolicyPort{
		Port: &port8000,
	}

	comment = craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)
	expectedComment = "PORT-8000-OF-"

	if !reflect.DeepEqual(comment, expectedComment) {
		t.Errorf("TestCraftPartialIptablesCommentFromPort failed @ port 8000 comment comparison")
		t.Errorf("comment:\n%v", comment)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedComment)
	}

	portRule = networkingv1.NetworkPolicyPort{
		Protocol: &tcp,
		Port:     &port8000,
	}

	comment = craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)
	expectedComment = "TCP-PORT-8000-OF-"

	if !reflect.DeepEqual(comment, expectedComment) {
		t.Errorf("TestCraftPartialIptablesCommentFromPort failed @ tcp port 8000 comment comparison")
		t.Errorf("comment:\n%v", comment)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedComment)
	}
}

func TestCraftPartialIptEntrySpecFromOpAndLabel(t *testing.T) {
	srcOp, srcLabel := "", "src"
	iptEntrySpec := craftPartialIptEntrySpecFromOpAndLabel(srcOp, srcLabel, util.IptablesSrcFlag, false)
	expectedIptEntrySpec := []string{
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName(srcLabel),
		util.IptablesSrcFlag,
	}

	if !reflect.DeepEqual(iptEntrySpec, expectedIptEntrySpec) {
		t.Errorf("TestCraftIptEntrySpecFromOpAndLabel failed @ src iptEntrySpec comparison")
		t.Errorf("iptEntrySpec:\n%v", iptEntrySpec)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedIptEntrySpec)
	}

	dstOp, dstLabel := "!", "dst"
	iptEntrySpec = craftPartialIptEntrySpecFromOpAndLabel(dstOp, dstLabel, util.IptablesDstFlag, false)
	expectedIptEntrySpec = []string{
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesNotFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName(dstLabel),
		util.IptablesDstFlag,
	}

	if !reflect.DeepEqual(iptEntrySpec, expectedIptEntrySpec) {
		t.Errorf("TestCraftIptEntrySpecFromOpAndLabel failed @ dst iptEntrySpec comparison")
		t.Errorf("iptEntrySpec:\n%v", iptEntrySpec)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedIptEntrySpec)
	}
}

func TestCraftPartialIptEntrySpecFromOpsAndLabels(t *testing.T) {
	srcOps := []string{
		"",
		"",
		"!",
	}
	srcLabels := []string{
		"src",
		"src:firstLabel",
		"src:secondLabel",
	}

	dstOps := []string{
		"!",
		"!",
		"",
	}
	dstLabels := []string{
		"dst",
		"dst:firstLabel",
		"dst:secondLabel",
	}

	srcIptEntry := craftPartialIptEntrySpecFromOpsAndLabels("testnamespace", srcOps, srcLabels, util.IptablesSrcFlag, false)
	dstIptEntry := craftPartialIptEntrySpecFromOpsAndLabels("testnamespace", dstOps, dstLabels, util.IptablesDstFlag, false)
	iptEntrySpec := append(srcIptEntry, dstIptEntry...)
	expectedIptEntrySpec := []string{
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName("src"),
		util.IptablesSrcFlag,
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName("src:firstLabel"),
		util.IptablesSrcFlag,
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesNotFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName("src:secondLabel"),
		util.IptablesSrcFlag,
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesNotFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName("dst"),
		util.IptablesDstFlag,
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesNotFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName("dst:firstLabel"),
		util.IptablesDstFlag,
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName("dst:secondLabel"),
		util.IptablesDstFlag,
	}

	if !reflect.DeepEqual(iptEntrySpec, expectedIptEntrySpec) {
		t.Errorf("TestCraftIptEntrySpecFromOpsAndLabels failed @ iptEntrySpec comparison")
		t.Errorf("iptEntrySpec:\n%v", iptEntrySpec)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedIptEntrySpec)
	}
}

func TestCraftPartialIptEntryFromSelector(t *testing.T) {
	srcSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"label": "src",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			metav1.LabelSelectorRequirement{
				Key:      "labelNotIn",
				Operator: metav1.LabelSelectorOpNotIn,
				Values: []string{
					"src",
				},
			},
		},
	}

	iptEntrySpec := craftPartialIptEntrySpecFromSelector("testnamespace", srcSelector, util.IptablesSrcFlag, false)
	expectedIptEntrySpec := []string{
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName("label:src"),
		util.IptablesSrcFlag,
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		util.IptablesNotFlag,
		util.IptablesMatchSetFlag,
		util.GetHashedName("labelNotIn:src"),
		util.IptablesSrcFlag,
	}

	if !reflect.DeepEqual(iptEntrySpec, expectedIptEntrySpec) {
		t.Errorf("TestCraftPartialIptEntryFromSelector failed @ iptEntrySpec comparison")
		t.Errorf("iptEntrySpec:\n%v", iptEntrySpec)
		t.Errorf("expectedIptEntrySpec:\n%v", expectedIptEntrySpec)
	}
}

func TestCraftPartialIptablesCommentFromSelector(t *testing.T) {
	var selector *metav1.LabelSelector
	selector = nil
	comment := craftPartialIptablesCommentFromSelector("testnamespace", selector, false)
	expectedComment := "none"
	if comment != expectedComment {
		t.Errorf("TestCraftPartialIptablesCommentFromSelector failed @ nil selector comparison")
		t.Errorf("comment:\n%v", comment)
		t.Errorf("expectedComment:\n%v", expectedComment)
	}

	selector = &metav1.LabelSelector{}
	comment = craftPartialIptablesCommentFromSelector("testnamespace", selector, false)
	expectedComment = "ns-testnamespace"
	if comment != expectedComment {
		t.Errorf("TestCraftPartialIptablesCommentFromSelector failed @ empty podSelector comparison")
		t.Errorf("comment:\n%v", comment)
		t.Errorf("expectedComment:\n%v", expectedComment)
	}

	comment = craftPartialIptablesCommentFromSelector("testnamespace", selector, true)
	expectedComment = util.KubeAllNamespacesFlag
	if comment != expectedComment {
		t.Errorf("TestCraftPartialIptablesCommentFromSelector failed @ empty namespaceSelector comparison")
		t.Errorf("comment:\n%v", comment)
		t.Errorf("expectedComment:\n%v", expectedComment)
	}

	selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"k0": "v0",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			metav1.LabelSelectorRequirement{
				Key:      "k1",
				Operator: metav1.LabelSelectorOpIn,
				Values: []string{
					"v10",
					"v11",
				},
			},
			metav1.LabelSelectorRequirement{
				Key:      "k2",
				Operator: metav1.LabelSelectorOpDoesNotExist,
				Values:   []string{},
			},
		},
	}
	comment = craftPartialIptablesCommentFromSelector("testnamespace", selector, false)
	expectedComment = "k0:v0-AND-k1:v10-AND-k1:v11-AND-!k2"
	if comment != expectedComment {
		t.Errorf("TestCraftPartialIptablesCommentFromSelector failed @ normal selector comparison")
		t.Errorf("comment:\n%v", comment)
		t.Errorf("expectedComment:\n%v", expectedComment)
	}

	nsSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"k0": "v0",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			metav1.LabelSelectorRequirement{
				Key:      "k1",
				Operator: metav1.LabelSelectorOpIn,
				Values: []string{
					"v10",
					"v11",
				},
			},
			metav1.LabelSelectorRequirement{
				Key:      "k2",
				Operator: metav1.LabelSelectorOpDoesNotExist,
				Values:   []string{},
			},
		},
	}
	comment = craftPartialIptablesCommentFromSelector("testnamespace", nsSelector, true)
	expectedComment = "ns-k0:v0-AND-ns-k1:v10-AND-ns-k1:v11-AND-ns-!k2"
	if comment != expectedComment {
		t.Errorf("TestCraftPartialIptablesCommentFromSelector failed @ namespace selector comparison")
		t.Errorf("comment:\n%v", comment)
		t.Errorf("expectedComment:\n%v", expectedComment)
	}

}
