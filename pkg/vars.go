package pkg

var (
	PendingDeploys chan DeployRequest
	ActiveSIGs     *SIGCache
)

const (
	CtxKey_DeletePartition CtxKeyType = "delete_partition"
	CtxKey_CreatePartition CtxKeyType = "create_partition"
)
