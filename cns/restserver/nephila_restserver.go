package restserver

import (
	"fmt"
	"net/http"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/nephila"
)

func (service *httpRestService) setNephilaConfig(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] setNephilaConfig")

	var res cns.NephilaNodeConfigResponse
	var req cns.NephilaNodeConfigRequest
	var nodeConfig nephila.NephilaNodeConfig

	returnMessage := ""
	returnCode := 0

	err := service.Listener.Decode(w, r, &req)
	if err != nil {
		return
	}

	service.lock.Lock()

	provider, err := nephila.NewNephilaProvider(req.Type)
	if err != nil {
		returnCode = UnexpectedError
		returnMessage = fmt.Sprintf("Failed to configure provider with error: %s", err.Error())
		goto Respond
	}
	nodeConfig, err = provider.ConfigureNode(req.NodeConfig, req.DNCConfig)
	if err != nil {
		returnCode = UnexpectedError
		returnMessage = fmt.Sprintf("Failed to configure node with error: %s", err.Error())
		goto Respond
	}

	service.state.NephilaType = req.Type

Respond:
	res.Config = nodeConfig
	service.saveState()
	service.lock.Unlock()

	response := cns.Response{
		ReturnCode: returnCode,
		Message:    returnMessage,
	}

	// add cns response to Nephila config response
	res.Response = response
	err = service.Listener.Encode(w, &res)
	log.Response(service.Name, res, err)
}
