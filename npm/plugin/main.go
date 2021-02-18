// Copyright 2018 Microsoft. All rights reserved.
// MIT License
package main

import (
	"time"

	_ "net/http/pprof"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/npm"
	restserver "github.com/Azure/azure-container-networking/npm/http/server"
	"github.com/Azure/azure-container-networking/npm/metrics"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const waitForTelemetryInSeconds = 60

// Version is populated by make during build.
var version string

func initLogging() error {
	log.SetName("azure-npm")
	log.SetLevel(log.LevelInfo)
	if err := log.SetTargetLogDirectory(log.TargetStdOutAndLogFile, ""); err != nil {
		log.Logf("Failed to configure logging, err:%v.", err)
		return err
	}

	return nil
}

func main() {
	var err error

	defer func() {
		if r := recover(); r != nil {
			log.Logf("recovered from error: %v", err)
		}
	}()

	if err = initLogging(); err != nil {
		panic(err.Error())
	}

	metrics.InitializeAll()

	// Creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// Creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Logf("clientset creation failed with error %v.", err)
		panic(err.Error())
	}

	factory := informers.NewSharedInformerFactory(clientset, time.Hour*24)

	npMgr := npm.NewNetworkPolicyManager(clientset, factory, version)
	metrics.CreateTelemetryHandle(npMgr.GetAppVersion(), npm.GetAIMetadata())

	// start NPM HTTP endpoint for Prometheus, pprof and debugging
	restserver := restserver.NewNpmRestServer(restserver.DefaultHTTPListeningAddress)
	go restserver.NPMRestServerListenAndServe(npMgr)

	//go npMgr.SendClusterMetrics()

	if err = npMgr.Start(wait.NeverStop); err != nil {
		log.Logf("npm failed with error %v.", err)
		panic(err.Error)
	}

	select {}
}
