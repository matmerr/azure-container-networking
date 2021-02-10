package api

const (
	DefaultListeningAddress = "0.0.0.0"
	DefaultHttpPort         = "10091"
	NodeMetricsPath         = "/node-metrics"
	ClusterMetricsPath      = "/cluster-metrics"
	InformersPath           = "/npm/v1/debug/namespaces"
)

type DescribeIPSetRequest struct {
	ipsetname string `json:"name"`
}

type DescribeIPSetResponse struct {
}
