package main

import (
	"fmt"
	"os"
	"testing"
)

const (
	testSourceDir     = "/home/matmerr/go/src/github.com/Azure/azure-container-networking/output/"
	testSourceFileDir = "/home/matmerr/go/src/github.com/Azure/azure-container-networking/output/linux_amd64/cni/"
	testOutputDir     = "/home/matmerr/go/src/github.com/Azure/azure-container-networking/bin/"
	testConflistDir   = "/home/matmerr/go/src/github.com/Azure/azure-container-networking/bin/"
)

func TestEnv(t *testing.T) {
	os.Setenv(envCNIOS, linux)
	os.Setenv(envCNITYPE, singletenancy)
	os.Setenv(envCNISourceDir, testSourceDir)
	os.Setenv(envCNIDestinationBinDir, testOutputDir)
	os.Setenv(envCNIIPAMType, azureCNSIPAM)
	os.Setenv(envCNIDestinationConflistDir, testConflistDir)

	envs, err := getDirectoriesFromEnv()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if envs.srcDir != testSourceFileDir {
		t.Fatalf("Directories don't match, %s, %s", envs.srcDir, testSourceFileDir)
	}

	main()
}
