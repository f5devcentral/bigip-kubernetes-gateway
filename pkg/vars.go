package pkg

import (
	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	"gitee.com/zongzw/f5-bigip-rest/utils"
)

var (
	PendingDeploy chan *[]f5_bigip.RestRequest
	slog          utils.SLOG
)
