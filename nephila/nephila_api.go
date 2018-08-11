package nephila

const (
	Type       = "Type"
	Flannel    = "Flannel"
	Disabled   = "Disabled"
	Config     = "Config"
	NodeConfig = "NodeConfig"
)

type NephilaProvider interface {
	GetType() string
	ConfigureNode(nodeConf NephilaNodeConfig, dncConf NephilaDNCConfig) (NephilaNodeConfig, error)
}

type NephilaDNCConfig struct {
	Type   string
	Config interface{}
}

type NephilaNodeConfig struct {
	Type   string
	Config interface{}
}

type NephilaNetworkContainerConfig struct {
	Type       string
	Config     interface{}
	NodeConfig interface{}
}
