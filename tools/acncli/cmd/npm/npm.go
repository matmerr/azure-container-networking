package npm

import (
	"fmt"

	"github.com/Azure/azure-container-networking/npm/http/api"

	c "github.com/Azure/azure-container-networking/hack/acncli/api"
	"github.com/Azure/azure-container-networking/hack/acncli/cmd/npm/get"
	npm "github.com/Azure/azure-container-networking/npm/http/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewRootCmd returns a root
func NPMRootCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "npm",
		Short: "Collection of functions related to Azure NPM",
	}

	viper.New()
	viper.SetEnvPrefix(c.EnvPrefix)
	viper.AutomaticEnv()

	npmEndpoint := fmt.Sprintf("%s:%s", "http://localhost", api.DefaultHttpPort)
	npmClient := npm.NewNPMHttpClient(npmEndpoint)

	cmd.AddCommand(GetCmd(npmClient))
	return cmd
}

func GetCmd(npmClient *npm.NPMHttpClient) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "get",
		Short: "Get in-memory maps from Azure NPM",
	}

	cmd.AddCommand(get.GetManagerCmd(npmClient))
	return cmd
}
