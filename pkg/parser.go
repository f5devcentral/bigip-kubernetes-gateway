package pkg

import (
	"fmt"
	"strings"

	"gitee.com/zongzw/f5-bigip-rest/utils"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func ParseHTTPRoute(hr *gatewayv1beta1.HTTPRoute) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()
	return nil, nil
}

func ParseGateway(gw *gatewayv1beta1.Gateway) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()
	if gw == nil {
		return map[string]interface{}{}, nil
	}

	ress := map[string]interface{}{}
	for _, addr := range gw.Spec.Addresses {
		if *addr.Type == gatewayv1beta1.IPAddressType {
			ipaddr := addr.Value
			for _, listener := range gw.Spec.Listeners {
				destination := fmt.Sprintf("%s:%d", ipaddr, listener.Port)
				if utils.IsIpv6(ipaddr) {
					destination = fmt.Sprintf("%s.%d", ipaddr, listener.Port)
				}
				name := strings.Join([]string{gw.Namespace, gw.Name, string(listener.Name)}, ".")
				profiles := map[string]interface{}{
					"items": []map[string]string{
						{"name": "http"},
					},
				}
				ipProtocol := "tcp"
				ress["ltm/virtual/"+name] = map[string]interface{}{
					"name":              name,
					"profilesReference": profiles,
					"ipProtocol":        ipProtocol,
					"destination":       destination,
				}
			}
		} else {
			return map[string]interface{}{}, fmt.Errorf("unsupported AddressType: %s", *addr.Type)
		}
	}

	cfgs := map[string]interface{}{
		"": ress,
	}
	return cfgs, nil
}
