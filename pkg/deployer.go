package pkg

import (
	"fmt"

	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
)

var (
	PendingDeploy chan *[]f5_bigip.RestRequest
)

func init() {
	PendingDeploy = make(chan *[]f5_bigip.RestRequest, 16)
}

func deploy(cmds *[]f5_bigip.RestRequest) {
	for _, cmd := range *cmds {
		fmt.Printf("cmd url: %s %v:", cmd.ResUri, cmd)
	}
}

func Deployer(stopCh chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		case cmds := <-PendingDeploy:
			deploy(cmds)
		}
	}
}
