package nephila

import (
	"encoding/json"
	"fmt"
)

func (n *NephilaNetworkContainerConfig) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	m[Type] = n.Type
	m[Config] = n.Config
	m[NodeConfig] = n.NodeConfig
	return json.Marshal(m)
}

func (n *NephilaNetworkContainerConfig) UnmarshalJSON(b []byte) error {
	m := make(map[string]interface{})
	json.Unmarshal(b, &m)
	confType := m[Type].(string)

	if confType == Flannel {
		n.Type = confType
		var ncc FlannelNetworkContainerConfig
		b, _ := json.Marshal(m[Config])
		json.Unmarshal(b, &ncc)
		n.Config = ncc
		var nnc FlannelNodeConfig
		b, _ = json.Marshal(m[NodeConfig])
		json.Unmarshal(b, &ncc)
		n.NodeConfig = nnc
		return nil
	}
	return fmt.Errorf("Failed to unmarshal NC config: %s", string(b))
}

func (n *NephilaNodeConfig) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	m[Type] = n.Type
	m[Config] = n.Config
	return json.Marshal(m)
}

func (n *NephilaNodeConfig) UnmarshalJSON(b []byte) error {
	m := make(map[string]interface{})
	json.Unmarshal(b, &m)
	confType := m[Type].(string)

	if confType == Flannel {
		n.Type = confType
		var ncc FlannelNodeConfig
		b, _ := json.Marshal(m[Config])
		json.Unmarshal(b, &ncc)
		n.Config = ncc
		return nil
	}
	return fmt.Errorf("Failed to unmarshal NC config: %s", string(b))
}

func (n *NephilaDNCConfig) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	m[Type] = n.Type
	m[Config] = n.Config
	return json.Marshal(m)
}

func (n *NephilaDNCConfig) UnmarshalJSON(b []byte) error {
	m := make(map[string]interface{})
	json.Unmarshal(b, &m)
	confType := m[Type].(string)

	if confType == Flannel {
		n.Type = confType
		var ncc FlannelDNCConfig
		b, _ := json.Marshal(m[Config])
		json.Unmarshal(b, &ncc)
		n.Config = ncc
		return nil
	}
	return fmt.Errorf("Failed to unmarshal NC config: %s", string(b))
}
