package main

import (
	"infra-sshfp-cf/cloudflare"
	"infra-sshfp-cf/config"
	"infra-sshfp-cf/consul"
	"infra-sshfp-cf/sshfp"
	"infra-sshfp-cf/statestore"

	"github.com/sirupsen/logrus"
)

func main() {
	//Debug loglevel
	logrus.SetLevel(logrus.InfoLevel)

	//Create configuration components
	cfgService := config.NewService(config.NewFileRepository())
	config, err := cfgService.LoadConfig("testcfg")
	if err != nil {
		logrus.Fatal(err)
	}

	//Create cloudflare components
	cloudflare := cloudflare.NewService(cloudflare.NewRepository(config.CloudflareToken, config.DomainName))

	//Create STDIN Listener and start listening on (blocking operation)
	consul := consul.NewService(consul.NewStdinRepository())
	err = consul.LoadData()

	//Code below is executed upon data receipt

	if err != nil {
		logrus.Fatal(err)
	}

	hosts := consul.GetHostnames()

	//Open statestore
	statestore := statestore.NewService(statestore.NewMapRepository("statestore.json"))

	//Iterate over hosts and check modify indexes
	for _, hostname := range hosts {
		modifiedIndex := consul.GetModifiedIndex(hostname)
		modified, _ := statestore.CheckIfModified(hostname, modifiedIndex)

		if modified {
			//Check if hosts has A record in CF - if not ignore
			exists, err := cloudflare.FindHostByName(hostname)
			if err != nil || !exists {
				continue
			}

			//Generate DNS records based on metadata
			sshfp := sshfp.NewService()
			consulKeys := sshfp.ParseConsulSSHRecords(consul.GetMetaData(hostname))

			//GetCurrent configuration
			cloudflareKeys, err := cloudflare.GetSSHFPRecordsForHost(hostname)
			if err != nil {
				logrus.Error(err)
			}

			configPlan := sshfp.PrepareConfiguration(hostname, cloudflareKeys, consulKeys)

			//ConfigPlan is empty, but host was flagged as modified. It's very likely that is a new host, or db is corrupted
			if len(configPlan) == 0 {
				statestore.SaveState(hostname, modifiedIndex)
			}

			if len(configPlan) > 0 {
				sshfp.PrintConfigPlan(configPlan)
				itemsApplied, err := cloudflare.ApplyConfigPlan(configPlan)
				if err != nil {
					logrus.Error(err)
				}
				if itemsApplied == len(configPlan) {
					statestore.SaveState(hostname, modifiedIndex)
				}
			}
		}
	}

}