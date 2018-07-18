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

	var dnc nephila.NephilaDNCConfig
	var nodeConfigResponse cns.NephilaNodeConfigResponse
	var nodeConfig nephila.NephilaNodeConfig

	returnMessage := ""
	returnCode := 0

	err := service.Listener.Decode(w, r, &dnc)
	if err != nil {
		return
	}

	service.lock.Lock()

	provider, err := nephila.NewNephilaProvider(dnc.Type)
	if err != nil {
		returnCode = UnexpectedError
		returnMessage = fmt.Sprintf("[Azure CNS Nephila] Failed to configure provider with error: %s", err.Error())
		goto Respond
	}
	nodeConfig, err = provider.ConfigureNode(dnc.Config)
	if err != nil {
		returnCode = UnexpectedError
		returnMessage = fmt.Sprintf("[Azure CNS Nephila] Failed to configure node with error: %s", err.Error())
		goto Respond
	}

	service.state.NephilaType = dnc.Type

Respond:
	nodeConfigResponse.Config = nodeConfig
	service.saveState()
	service.lock.Unlock()

	resp := cns.Response{
		ReturnCode: returnCode,
		Message:    returnMessage,
	}

	// add cns response to Nephila config response
	nodeConfigResponse.Response = resp

	err = service.Listener.Encode(w, &nodeConfigResponse)
	log.Response(service.Name, resp, err)
}
