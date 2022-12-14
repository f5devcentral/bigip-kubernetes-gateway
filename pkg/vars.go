package pkg

var (
	PendingDeploys  chan DeployRequest
	PendingParses   chan ParseRequest
	ActiveSIGs      *SIGCache
	AllBigipConfigs BigipConfigs
	// slog            utils.SLOG
)

const (
	CtxKey_DeletePartition CtxKeyType = "delete_partition"
	CtxKey_CreatePartition CtxKeyType = "create_partition"
)
