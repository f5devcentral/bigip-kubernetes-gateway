package pkg

import (
	f5_bigip "github.com/zongzw/f5-bigip-rest/bigip"
	"github.com/zongzw/f5-bigip-rest/deployer"
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
