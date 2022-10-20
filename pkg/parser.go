package pkg

import (
	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	"gitee.com/zongzw/f5-bigip-rest/utils"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func ParseHTTPRoute(hr *gatewayv1beta1.HTTPRoute) []f5_bigip.RestRequest {
	defer utils.TimeItToPrometheus()()
	return []f5_bigip.RestRequest{}
}

func ParseGateway(gw *gatewayv1beta1.Gateway) []f5_bigip.RestRequest {
	defer utils.TimeItToPrometheus()()
	return []f5_bigip.RestRequest{}
}
