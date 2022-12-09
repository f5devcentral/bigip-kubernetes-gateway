package pkg

var (
	PendingDeploys  chan DeployRequest
	PendingParses   chan ParseRequest
	ActiveSIGs      *SIGCache
	AllBigipConfigs BigipConfigs
	// slog            utils.SLOG
)
