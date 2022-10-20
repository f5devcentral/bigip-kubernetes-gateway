package pkg

import (
	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	"gitee.com/zongzw/f5-bigip-rest/utils"
)

func init() {
	PendingDeploy = make(chan *map[string]interface{}, 16)
	slog = utils.SetupLog("", "debug")
}

func deploy(bigip *f5_bigip.BIGIP, cfgs *map[string]interface{}) error {
	defer utils.TimeItToPrometheus()()

	slog.Debugf("deploying %d resources to bigip: %s", len(*cfgs), bigip.URL)
	for fn, res := range *cfgs {
		slog.Infof("cfg: %s %v:", fn, res)
	}
	cmds, err := bigip.GenRestRequests("cis-c-tenant", nil, cfgs)
	if err != nil {
		return err
	}
	return bigip.DoRestRequests(cmds)
}

func Deployer(stopCh chan struct{}, bigip *f5_bigip.BIGIP) {
	for {
		select {
		case <-stopCh:
			return
		case cfgs := <-PendingDeploy:
			err := deploy(bigip, cfgs)
			if err != nil {
				// report the error to status or ...
			}
		}
	}
}
