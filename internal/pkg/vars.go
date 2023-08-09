package pkg

import (
	f5_bigip "github.com/f5devcentral/f5-bigip-rest-go/bigip"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
)

var (
	PendingDeploys *utils.DeployQueue
	DoneDeploys    *utils.DeployQueue
	ActiveSIGs     *SIGCache
	BIGIPs         []*f5_bigip.BIGIP
	BIPConfigs     BIGIPConfigs
	BIPPassword    string
	refFromTo      *ReferenceGrantFromTo
	LogLevel       string
)

// const (
// 	DeployMethod_AS3  = "as3"
// 	DeployMethod_REST = "rest"
// )
