package pkg

import f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"

var (
	PendingDeploys chan DeployRequest
	ActiveSIGs     *SIGCache
	BIGIPs         []*f5_bigip.BIGIP
	BIPConfigs     BIGIPConfigs
	BIPPassword    string
)

const (
	CtxKey_DeletePartition CtxKeyType = "delete_partition"
	CtxKey_CreatePartition CtxKeyType = "create_partition"
	CtxKey_SpecifiedBIGIP  CtxKeyType = "specified_bigip"
)
