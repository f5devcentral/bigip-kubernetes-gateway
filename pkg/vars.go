package pkg

import (
	"gitee.com/zongzw/f5-bigip-rest/utils"
)

var (
	PendingDeploys chan DeployRequest
	slog           utils.SLOG
	ActiveSIGs     *SIGCache
)
