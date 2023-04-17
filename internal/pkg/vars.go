package pkg

import (
	f5_bigip "github.com/f5devcentral/f5-bigip-rest-go/bigip"
	"github.com/f5devcentral/f5-bigip-rest-go/deployer"
)

var (
	PendingDeploys chan deployer.DeployRequest
	DoneDeploys    *deployer.DeployResponses
	ActiveSIGs     *SIGCache
	BIGIPs         []*f5_bigip.BIGIP
	BIPConfigs     BIGIPConfigs
	BIPPassword    string
	refFromTo      *ReferenceGrantFromTo
	LogLevel       string
)
