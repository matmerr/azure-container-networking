package get

import (
	"github.com/Azure/azure-container-networking/hack/acncli/api"
	"github.com/Azure/azure-container-networking/log"
	npm "github.com/Azure/azure-container-networking/npm/http/client"
	"github.com/spf13/cobra"
)

func GetManagerCmd(npmClient *npm.NPMHttpClient) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "npmgr",
		Short: "Get NPM in memory Namespace map",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespaces, err := npmClient.GetNpmMgr()
			if err == nil {
				api.PrettyPrint(namespaces)
			} else {
				log.Printf("err %v", err)
			}
			return err
		},
	}

	return cmd
}
