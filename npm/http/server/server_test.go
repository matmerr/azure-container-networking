package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Azure/azure-container-networking/npm/http/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Azure/azure-container-networking/npm"
)

func TestGetNpmMgrHandler(t *testing.T) {
	npMgr := &npm.NetworkPolicyManager{
		NsMap: map[string]*npm.Namespace{
			"test": &npm.Namespace{
				PodMap: map[types.UID]*corev1.Pod{
					"": &corev1.Pod{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "testpod",
						},
						Spec:   corev1.PodSpec{},
						Status: corev1.PodStatus{},
					},
				},
			},
		},
	}
	n := NewNpmRestServer(npMgr)
	handler := n.GetNpmMgr()

	req, err := http.NewRequest("GET", api.InformersPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var ns npm.NetworkPolicyManager
	err = json.NewDecoder(rr.Body).Decode(&ns)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ns, npMgr) {
		t.Fatalf("Expected: %+v,\n actual %+v", ns, npMgr)
	}
}
