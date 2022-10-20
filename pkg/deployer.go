package pkg

import (
	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	"gitee.com/zongzw/f5-bigip-rest/utils"
)

func deploy(bigip *f5_bigip.BIGIP, ocfgs, ncfgs *map[string]interface{}) error {
	defer utils.TimeItToPrometheus()()

	// slog.Debugf("deploying %d resources to bigip: %s", len(*ncfgs), bigip.URL)
	// for fn, res := range *cfgs {
	// 	slog.Infof("cfg: %s %v:", fn, res)
	// }
	cmds, err := bigip.GenRestRequests("cis-c-tenant", ocfgs, ncfgs)
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
		case r := <-PendingDeploys:
			err := deploy(bigip, r.From, r.To)
			if err != nil {
				// report the error to status or ...
				slog.Errorf("failed to do deployment: %s", err.Error())
			} else {

			}
		}
	}
}
