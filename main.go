package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	WardenClient "github.com/cloudfoundry-incubator/garden/client"
	WardenConnection "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"github.com/winston-ci/prole/api"
	"github.com/winston-ci/prole/builder"
	"github.com/winston-ci/prole/checker"
	"github.com/winston-ci/prole/config"
	"github.com/winston-ci/prole/outputter"
	"github.com/winston-ci/prole/scheduler"
	"github.com/winston-ci/prole/sourcefetcher"
)

var listenAddr = flag.String(
	"listenAddr",
	"0.0.0.0:4637",
	"listening address",
)

var wardenNetwork = flag.String(
	"wardenNetwork",
	"unix",
	"warden API connection network (unix or tcp)",
)

var wardenAddr = flag.String(
	"wardenAddr",
	"/tmp/warden.sock",
	"warden API connection address",
)

var resourceTypes = flag.String(
	"resourceTypes",
	`{"git":"winston/git-resource","raw":"winston/raw-resource"}`,
	"map of resource type to its docker image",
)

func main() {
	flag.Parse()

	wardenClient := WardenClient.New(&WardenConnection.Info{
		Network: *wardenNetwork,
		Addr:    *wardenAddr,
	})

	resourceTypesMap := map[string]string{}
	err := json.Unmarshal([]byte(*resourceTypes), &resourceTypesMap)
	if err != nil {
		log.Fatalln("failed to parse resource types:", err)
	}

	var resourceTypesConfig config.ResourceTypes
	for typ, image := range resourceTypesMap {
		resourceTypesConfig = append(resourceTypesConfig, config.ResourceType{
			Name:  typ,
			Image: image,
		})
	}

	sourceFetcher := sourcefetcher.NewSourceFetcher(resourceTypesConfig, wardenClient)
	outputter := outputter.NewOutputter(resourceTypesConfig, wardenClient)
	builder := builder.NewBuilder(sourceFetcher, outputter, wardenClient)

	checker := checker.NewChecker(resourceTypesConfig, wardenClient)

	scheduler := scheduler.NewScheduler(builder)

	handler, err := api.New(scheduler, checker)
	if err != nil {
		log.Fatalln("failed to initialize handler:", err)
	}

	group := grouper.EnvokeGroup(grouper.RunGroup{
		"api": http_server.New(*listenAddr, handler),
	})

	running := ifrit.Envoke(sigmon.New(group))

	log.Println("serving api on", *listenAddr)

	err = <-running.Wait()
	if err != nil {
		log.Println("exited with error:", err)
		os.Exit(1)
	}

	log.Println("exited")
}
