package cmd

import (
	"fmt"

<<<<<<< HEAD:tools/acncli/cmd/root.go
	c "github.com/Azure/azure-container-networking/tools/acncli/api"
=======
	"github.com/Azure/azure-container-networking/hack/acncli/cmd/npm"

	"github.com/Azure/azure-container-networking/hack/acncli/cmd/cni"

	c "github.com/Azure/azure-container-networking/hack/acncli/api"
>>>>>>> f84e1f2d (add npm debug api and add to acncli):hack/acncli/cmd/root.go
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewRootCmd returns a root
func NewRootCmd(version string) *cobra.Command {
	var rootCmd = &cobra.Command{
		SilenceUsage: true,
		Version:      version,
	}

	viper.New()
	viper.SetEnvPrefix(c.EnvPrefix)
	viper.AutomaticEnv()

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version for ACN CLI",
		Run: func(cmd *cobra.Command, args []string) {
			if version != "" {
				fmt.Printf("%+s", version)
			} else {
				fmt.Println("Version not set.")
			}
		},
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cni.CNICmd())
	rootCmd.AddCommand(npm.NPMRootCmd())
	rootCmd.SetVersionTemplate(version)
	return rootCmd
}
