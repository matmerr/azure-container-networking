package cnsclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/common"
	"github.com/Azure/azure-container-networking/cns/logger"
	"github.com/Azure/azure-container-networking/cns/restserver"
	"github.com/Azure/azure-container-networking/log"
)

func TestMain(m *testing.M) {
	tmpFileState, err := ioutil.TempFile(os.TempDir(), "cns-*.json")
	tmpLogDir, err := ioutil.TempDir("", "cns-")

	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(tmpLogDir)
	defer os.Remove(tmpFileState.Name())

	if err != nil {
		panic(err)
	}

	logger.InitLogger("azure-cns.log", 0, 0, tmpLogDir)
	config := common.ServiceConfig{}

	httpRestService, err := restserver.NewHTTPRestService(&config)
	if err != nil {
		logger.Errorf("Failed to create CNS object, err:%v.\n", err)
		return
	}

	if httpRestService != nil {
		err = httpRestService.Start(&config)
		if err != nil {
			logger.Errorf("Failed to start CNS, err:%v.\n", err)
			return
		}
	}

	m.Run()
	time.Sleep(30 * time.Second)
}

func TestSetOrchestratorType(t *testing.T) {
	var (
		info = &cns.SetOrchestratorTypeRequest{
			OrchestratorType: cns.Kubernetes}
		body bytes.Buffer
	)

	if err := json.NewEncoder(&body).Encode(info); err != nil {
		log.Errorf("encoding json failed with %v", err)
		return
	}

	httpc := &http.Client{}
	url := defaultCnsURL + cns.SetOrchestratorType

	res, err := httpc.Post(url, "application/json", &body)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(res)

}
