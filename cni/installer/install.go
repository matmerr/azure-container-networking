package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	cn "github.com/Azure/azure-container-networking/cni"
)

const (
	binPerm      = 755
	conflistPerm = 644

	linux   = "linux"
	windows = "windows"
	amd64   = "amd64"

	azureCNIBin             = "azure-vnet"
	azureTelemetryBin       = "azure-vnet-telemetry"
	azureCNSIPAM            = "azure-cns"
	auzureVNETIPAM          = "azure-vnet-ipam"
	conflistExtension       = ".conflist"
	cni                     = "cni"
	multitenancy            = "multitenancy"
	singletenancy           = "singletenancy"
	defaultSrcDirLinux      = "/output/"
	defaultBinDirLinux      = "/opt/cni/bin/"
	defaultConflistDirLinux = "/etc/cni/net.d/"

	envCNIOS                     = "CNI_OS"
	envCNITYPE                   = "CNI_TYPE"
	envCNISourceDir              = "CNI_SRC_DIR"
	envCNIDestinationBinDir      = "CNI_DST_BIN_DIR"
	envCNIDestinationConflistDir = "CNI_DST_CONFLIST_DIR"
	envCNIIPAMType               = "CNI_IPAM_TYPE"
)

type environmentalVariables struct {
	srcDir         string
	dstBinDir      string
	dstConflistDir string
	ipamType       string
}

var (
	version     string
	exemptFiles = map[string]bool{azureTelemetryBin: true}
)

func main() {
	envs, err := getDirectoriesFromEnv()
	if err != nil {
		fmt.Printf("Failed to get environmental variables with err: %v", err)
		os.Exit(1)
	}

	if _, err := os.Stat(envs.dstBinDir); os.IsNotExist(err) {
		os.MkdirAll(envs.dstBinDir, binPerm)
	}

	if _, err := os.Stat(envs.dstConflistDir); os.IsNotExist(err) {
		os.MkdirAll(envs.dstConflistDir, conflistPerm)
	}

	binaries, conflists, err := getFiles(envs.srcDir)
	if err != nil {
		fmt.Printf("Failed to get CNI related file paths with err: %v", err)
		os.Exit(1)
	}

	err = copyBinaries(binaries, envs.dstBinDir, binPerm)
	if err != nil {
		fmt.Printf("Failed to copy CNI binaries with err: %v", err)
		os.Exit(1)
	}

	for _, conf := range conflists {
		err = modifyConflists(conf, envs, conflistPerm)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	if version == "" {
		version = "[No version set]"
	}

	fmt.Printf("Successfully installed Azure CNI %s and binaries to %s and conflist to %s\n", version, envs.dstBinDir, envs.dstConflistDir)
}

func modifyConflists(conf string, envs environmentalVariables, perm os.FileMode) error {
	jsonFile, err := os.Open(conf)
	defer jsonFile.Close()
	if err != nil {
		return err
	}

	var conflist cn.NetworkConfig
	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal(byteValue, &conflist)

	if envs.ipamType != "" {
		// change conflist to ipam type
		conflist.Ipam.Type = envs.ipamType
	}

	// get target path
	dstFile := envs.dstConflistDir + filepath.Base(conf)
	file, err := json.MarshalIndent(conflist, "", " ")
	if err != nil {
		return err
	}

	fmt.Println(dstFile)
	err = ioutil.WriteFile(dstFile, file, perm)
	if err != nil {
		return err
	}

	return nil
}

func getDirectoriesFromEnv() (environmentalVariables, error) {
	osVersion := os.Getenv(envCNIOS)
	cniType := os.Getenv(envCNITYPE)
	srcDirectory := os.Getenv(envCNISourceDir)
	dstBinDirectory := os.Getenv(envCNIDestinationBinDir)
	dstConflistDirectory := os.Getenv(envCNIDestinationConflistDir)
	ipamType := os.Getenv(envCNIIPAMType)

	fmt.Printf("IPAM TYPE %s", ipamType)

	var envs environmentalVariables

	if strings.EqualFold(osVersion, linux) || strings.EqualFold(osVersion, windows) {
		osVersion = fmt.Sprintf("%s_%s", osVersion, amd64)
	} else {
		return envs, fmt.Errorf("No target OS version supplied, please set \"%s\" env and try again", envCNIOS)
	}

	switch {
	case strings.EqualFold(cniType, multitenancy):
		cniType = fmt.Sprintf("%s-%s", cni, multitenancy)
	case strings.EqualFold(cniType, singletenancy):
		cniType = cni
	default:
		return envs, fmt.Errorf("No CNI type supplied, please set \"%s\" env to either \"%s\" or \"%s\" and try again", envCNITYPE, singletenancy, multitenancy)
	}

	if srcDirectory == "" {
		srcDirectory = defaultSrcDirLinux
	}

	if dstBinDirectory == "" {
		dstBinDirectory = defaultBinDirLinux
	}

	if dstConflistDirectory == "" {
		dstConflistDirectory = defaultConflistDirLinux
	}

	// srcDirectory ends with a / because it is a directory
	srcDirectory = fmt.Sprintf("%s%s/%s/", srcDirectory, osVersion, cniType)

	envs = environmentalVariables{
		srcDir:         srcDirectory,
		dstBinDir:      dstBinDirectory,
		dstConflistDir: dstConflistDirectory,
		ipamType:       ipamType,
	}

	return envs, nil
}

func getFiles(path string) (binaries []string, conflists []string, err error) {
	err = filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("Failed to traverse path %s with err %s", path, err)
			}

			if !info.IsDir() {
				ext := filepath.Ext(path)
				if ext == conflistExtension {
					conflists = append(conflists, path)
				} else {
					binaries = append(binaries, path)
				}

			}

			return nil
		})

	return
}

func copyBinaries(filePaths []string, dstDirectory string, perm os.FileMode) error {
	for _, path := range filePaths {
		fileName := filepath.Base(path)

		if exempt, ok := exemptFiles[fileName]; ok && exempt {
			fmt.Printf("Skipping %s, marked as exempt\n", fileName)
		} else {
			err := copy(path, dstDirectory+fileName, perm)
			fmt.Println(dstDirectory + fileName)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func copy(src string, dst string, perm os.FileMode) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dst, data, perm)
	if err != nil {
		return err
	}

	return nil
}
