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

	if val, _ := m[Type]; val == "" {
		n.Type = Disabled
		return nil
	}
	confType := m[Type].(string)
	if confType == Flannel {
		n.Type = confType

		var ncconf FlannelNetworkContainerConfig
		b1, err := json.Marshal(m[Config])
		if err != nil {
			return fmt.Errorf("Failed to unmarshal Nephila NC config with error: %v", err)
		}
		json.Unmarshal(b1, &ncconf)
		n.Config = ncconf

		var nodeconf FlannelNodeConfig
		b2, err := json.Marshal(m[NodeConfig])
		if err != nil {
			return fmt.Errorf("Failed to unmarshal Nephila NC config with error: %v", err)
		}
		json.Unmarshal(b2, &nodeconf)
		n.NodeConfig = nodeconf
		return nil
	}
	return nil
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

	if val, _ := m[Type]; val == "" {
		n.Type = Disabled
		return nil
	}
	confType := m[Type].(string)
	if confType == Flannel {
		n.Type = confType
		var ncc FlannelNodeConfig
		b, err := json.Marshal(m[Config])
		if err != nil {
			return fmt.Errorf("Failed to unmarshal Nephila Node config with error: %v", err)
		}
		json.Unmarshal(b, &ncc)
		n.Config = ncc
		return nil
	}
	return nil
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

	if val, _ := m[Type]; val == "" {
		n.Type = Disabled
		return nil
	}
	confType := m[Type].(string)
	if confType == Flannel {
		n.Type = confType
		var ncc FlannelDNCConfig
		b, err := json.Marshal(m[Config])
		if err != nil {
			return fmt.Errorf("Failed to unmarshal Nephila DNC config with error: %v", err)
		}
		json.Unmarshal(b, &ncc)
		n.Config = ncc
		return nil
	}
	return nil
}
