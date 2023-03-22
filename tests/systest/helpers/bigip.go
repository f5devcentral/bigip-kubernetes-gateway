package helpers

import (
	"context"
	"fmt"

	f5_bigip "github.com/zongzw/f5-bigip-rest/bigip"
)

type BIGIPHelper struct {
	username  string
	password  string
	ipAddress string
	port      int
	BIGIP     *f5_bigip.BIGIP
}

func NewBIGIPHelper(username, password, ipAddress string, port int) *BIGIPHelper {
	url := fmt.Sprintf("https://%s:%d", ipAddress, port)
	bip := f5_bigip.New(url, username, password)
	return &BIGIPHelper{
		username:  username,
		password:  password,
		ipAddress: ipAddress,
		port:      port,
		BIGIP:     bip,
	}
}

func (bh *BIGIPHelper) Check(ctx context.Context, kind, name, partition, subfolder string, properties map[string]interface{}) error {
	bc := f5_bigip.BIGIPContext{BIGIP: *bh.BIGIP, Context: ctx}
	existings, err := bc.Exist(kind, name, partition, subfolder)
	if err != nil {
		return fmt.Errorf("failed to get resources: %s", err)
	} else if existings == nil {
		return fmt.Errorf("empty response from bigip with p(%s) n(%s), f(%s), k(%s)", partition, name, subfolder, kind)
	} else {
		for k, props := range properties {
			if !deepequal((*existings)[k], props) {
				return fmt.Errorf("expected: %v, actually: %v", props, (*existings)[k])
			}
		}
		return nil
	}
}

func (bh *BIGIPHelper) Exist(ctx context.Context, kind, name, partition, subfolder string) error {
	// slog := utils.LogFromContext(ctx)
	bc := f5_bigip.BIGIPContext{BIGIP: *bh.BIGIP, Context: ctx}
	existings, err := bc.Exist(kind, name, partition, subfolder)
	if err != nil {
		return fmt.Errorf("failed to get resources: %s", err)
	} else if existings == nil {
		return fmt.Errorf("empty response from bigip with p(%s) n(%s), f(%s), k(%s)", partition, name, subfolder, kind)
	} else {
		// slog.Infof("existings: %v", *existings)
		return nil
	}
}

func (bh *BIGIPHelper) Get(ctx context.Context, kind, name, partition, subfolder string) (map[string]interface{}, error) {
	bc := f5_bigip.BIGIPContext{BIGIP: *bh.BIGIP, Context: ctx}

	existings, err := bc.Exist(kind, name, partition, subfolder)
	if err != nil {
		return *existings, fmt.Errorf("failed to get resources: %s", err)
	} else if existings == nil {
		return nil, fmt.Errorf("empty response from bigip with p(%s) n(%s), f(%s), k(%s)", partition, name, subfolder, kind)
	} else {
		return *existings, nil
	}
}
