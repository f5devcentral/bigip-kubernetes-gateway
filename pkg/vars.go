package pkg

import (
	"gitee.com/zongzw/f5-bigip-rest/utils"
)

var (
	PendingDeploy chan *map[string]interface{}
	slog          utils.SLOG
)
