package pkg

import (
	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func Parse() {}

func ParseGateway(gw *gatewayv1beta1.Gateway) []f5_bigip.RestRequest {
	return []f5_bigip.RestRequest{}
}
