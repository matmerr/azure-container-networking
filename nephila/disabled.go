package nephila

// DisabledNephilaProvider is returned when the provider is set to "Disabled"
type DisabledNephilaProvider struct{}

func (fnp DisabledNephilaProvider) GetType() string {
	return Disabled
}

func (dnp DisabledNephilaProvider) ConfigureNode(nodeConf NephilaNodeConfig, dncConf NephilaDNCConfig) (NephilaNodeConfig, error) {
	nodeConfig := NephilaNodeConfig{
		Type: Disabled,
	}
	return nodeConfig, nil
}
