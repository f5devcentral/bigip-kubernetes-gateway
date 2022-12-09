package pkg

import (
	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	"gitee.com/zongzw/f5-bigip-rest/utils"
)

func deploy(bc *f5_bigip.BIGIPContext, partition string, ocfgs, ncfgs *map[string]interface{}) error {
	defer utils.TimeItToPrometheus()()

	if err := bc.DeployPartition(partition); err != nil {
		return err
	}

	cmds, err := bc.GenRestRequests(partition, ocfgs, ncfgs)
	if err != nil {
		return err
	}
	return bc.DoRestRequests(cmds)
	// if err := bigip.DoRestRequests(cmds); err != nil {
	// 	return err
	// }
	// if ncfgs == nil {
	// 	slog.Debugf("deleting partition: %s", partition)
	// 	return bigip.DeletePartition(partition)
	// }
	// return nil
}

// func filterPoolCfgs(ocfgs, ncfgs *map[string]interface{}) (*map[string]interface{}, *map[string]interface{}, error) {

// 	ocfgsPool := map[string]interface{}{}
// 	ncfgsPool := map[string]interface{}{}
// 	if ocfgs != nil {
// 		for fn, res := range *ocfgs {
// 			if _, f := ocfgsPool[fn]; !f {
// 				ocfgsPool[fn] = map[string]interface{}{}
// 			}
// 			fnmap := ocfgsPool[fn].(map[string]interface{})
// 			if resJson, ok := res.(map[string]interface{}); !ok {
// 				return nil, nil, fmt.Errorf("invalid resource format, should be json")
// 			} else {
// 				for tn, body := range resJson {
// 					if strings.HasPrefix(tn, "ltm/pool/") || strings.HasPrefix(tn, "ltm/arp/") || strings.HasPrefix(tn, "ltm/node/") {
// 						fnmap[tn] = body
// 					}
// 				}
// 			}
// 		}
// 	}
// 	if ncfgs != nil {
// 		for fn, res := range *ncfgs {
// 			if _, f := ncfgsPool[fn]; !f {
// 				ncfgsPool[fn] = map[string]interface{}{}
// 			}
// 			fnmap := ncfgsPool[fn].(map[string]interface{})
// 			if resJson, ok := res.(map[string]interface{}); !ok {
// 				return nil, nil, fmt.Errorf("invalid resource format, should be json")
// 			} else {
// 				for tn, body := range resJson {
// 					if strings.HasPrefix(tn, "ltm/pool/") || strings.HasPrefix(tn, "ltm/arp/") || strings.HasPrefix(tn, "ltm/node/") {
// 						fnmap[tn] = body
// 					}
// 				}
// 			}
// 		}
// 	}

// 	return &ocfgsPool, &ncfgsPool, nil
// }

func Deployer(stopCh chan struct{}, bigips []*f5_bigip.BIGIP) {
	for {
		select {
		case <-stopCh:
			return
		case r := <-PendingDeploys:
			slog := utils.LogFromContext(r.Context)
			slog.Debugf("Processing request: %s", r.Meta)
			for _, bigip := range bigips {
				bc := &f5_bigip.BIGIPContext{BIGIP: *bigip, Context: r.Context}
				err := deploy(bc, r.Partition, r.From, r.To)
				if err != nil {
					// report the error to status or ...
					slog.Errorf("failed to do deployment: %s", err.Error())
				} else {
					r.StatusFunc()
				}
			}
		}
	}
}

func ModifyDbValue(bc *f5_bigip.BIGIPContext) error {
	//tmrouted.tmos.routing
	slog := utils.LogFromContext(bc)
	slog.Debugf("enabing tmrouted.tmos.routing ")
	return bc.ModifyDbValue("tmrouted.tmos.routing", "enable")
}

func ConfigFlannel(bc *f5_bigip.BIGIPContext, vxlanProfileName, vxlanPort, vxlanTunnelName, vxlanLocalAddress, selfIpName, selfIpAddress string) error {
	slog := utils.LogFromContext(bc)
	slog.Debugf("adding some flannel related configs onto bigip")
	err := bc.CreateVxlanProfile(vxlanProfileName, vxlanPort)
	if err != nil {
		return err
	}

	err = bc.CreateVxlanTunnel(vxlanTunnelName, "1", vxlanLocalAddress, vxlanProfileName)
	if err != nil {
		return err
	}

	err = bc.CreateSelf(selfIpName, selfIpAddress, vxlanTunnelName)
	if err != nil {
		return err
	}
	return nil
}
