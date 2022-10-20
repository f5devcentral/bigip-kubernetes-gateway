package pkg

import (
	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	"gitee.com/zongzw/f5-bigip-rest/utils"
)

func init() {
	PendingDeploy = make(chan *[]f5_bigip.RestRequest, 16)
	slog = utils.SetupLog("", "debug")
}

func deploy(bigip *f5_bigip.BIGIP, cmds *[]f5_bigip.RestRequest) {
	defer utils.TimeItToPrometheus()()
	slog.Debugf("deploying %d resources to bigip: %s", len(*cmds), bigip.URL)
	for _, cmd := range *cmds {
		slog.Infof("cmd url: %s %v:", cmd.ResUri, cmd)
	}
}

func Deployer(stopCh chan struct{}, bigip *f5_bigip.BIGIP) {
	for {
		select {
		case <-stopCh:
			return
		case cmds := <-PendingDeploy:
			deploy(bigip, cmds)
		}
	}
}
