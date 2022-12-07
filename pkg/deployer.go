package pkg

import (
	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	"gitee.com/zongzw/f5-bigip-rest/utils"
)

func deploy(bigip *f5_bigip.BIGIP, partition string, ocfgs, ncfgs *map[string]interface{}) error {
	defer utils.TimeItToPrometheus()()

	if err := bigip.DeployPartition(partition); err != nil {
		return err
	}

	// // filter out pools arps nodes from ocfgs and ncfgs
	// opcfgs, npcfgs, err := filterPoolCfgs(ocfgs, ncfgs)
	// if err != nil {
	// 	return err
	// }

	// // case: pools arps and nodes are in cis-c-tenant
	// pcmds, err := bigip.GenRestRequests("cis-c-tenant", opcfgs, npcfgs)
	// for
	// // 	for pools to delete, check if there's no refs to them, collect arps and nodes, delete them.
	// // 	for pools to create, collect arps and nodes to create them.

	// // case: pools arps and nodes are in namespace partition
	// // 	for pools to delete, check if there's no refs to them,
	// // 		delete pool and nodes from namespace partition
	// // 		delete arps from cis-c-tenant
	// // 	for pools to create,
	// // 		create pool and nodes to namepsace partition
	// // 		create arps to cis-c-tenent
	cmds, err := bigip.GenRestRequests(partition, ocfgs, ncfgs)
	if err != nil {
		return err
	}
	return bigip.DoRestRequests(cmds)
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

func Deployer(stopCh chan struct{}, bigip *f5_bigip.BIGIP) {
	for {
		select {
		case <-stopCh:
			return
		case r := <-PendingDeploys:
			slog.Debugf("Processing request: %s", r.Meta)
			err := deploy(bigip, r.Partition, r.From, r.To)
			if err != nil {
				// report the error to status or ...
				slog.Errorf("failed to do deployment: %s", err.Error())
			} else {
				r.StatusFunc()
			}
		}
	}
}

func ModifyDbValue(bigip *f5_bigip.BIGIP) error {
	//tmrouted.tmos.routing
	slog.Debugf("enabing tmrouted.tmos.routing ")
	return bigip.ModifyDbValue("tmrouted.tmos.routing", "enable")
}

func ConfigFlannel(bigip *f5_bigip.BIGIP, vxlanProfileName, vxlanPort, vxlanTunnelName, vxlanLocalAddress, selfIpName, selfIpAddress string) error {
	slog.Debugf("adding some flannel related configs onto bigip")
	err := bigip.CreateVxlanProfile(vxlanProfileName, vxlanPort)
	if err != nil {
		return err
	}

	err = bigip.CreateVxlanTunnel(vxlanTunnelName, "1", vxlanLocalAddress, vxlanProfileName)
	if err != nil {
		return err
	}

	err = bigip.CreateSelf(selfIpName, selfIpAddress, vxlanTunnelName)
	if err != nil {
		return err
	}
	return nil
}
