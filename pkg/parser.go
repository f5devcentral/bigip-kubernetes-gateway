package pkg

import (
	"fmt"
	"strings"

	"gitee.com/zongzw/bigip-kubernetes-gateway/k8s"
	"gitee.com/zongzw/f5-bigip-rest/utils"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func ParseHTTPRoute(hr *gatewayv1beta1.HTTPRoute) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	if hr == nil {
		return map[string]interface{}{}, nil
	}

	rlt := map[string]interface{}{}

	// pools from backendRefs
	for _, rl := range hr.Spec.Rules {
		for _, br := range rl.BackendRefs {
			ns := hr.Namespace
			if br.Namespace != nil {
				ns = string(*br.Namespace)
			}
			name := strings.Join([]string{ns, string(br.Name)}, ".")
			rlt["ltm/pool/"+name] = map[string]interface{}{
				"name":    name,
				"monitor": "min 1 of http tcp",
				"members": []interface{}{},

				// "minActiveMembers": 0,
				// TODO: there's at least one field for PATCH. or we may need to fix that
				// {"code":400,"message":"transaction failed:one or more properties must be specified","errorStack":[],"apiError":2}
			}
			svc := ActiveSIGs.GetService(utils.Keyname(ns, string(br.Name)))
			eps := ActiveSIGs.GetEndpoints(utils.Keyname(ns, string(br.Name)))
			if svc != nil && eps != nil {
				if mbs, err := k8s.FormatMembersFromServiceEndpoints(svc, eps); err != nil {
					return map[string]interface{}{}, err
				} else {
					fmtmbs := []interface{}{}

					for _, mb := range mbs {
						sep := ":"
						if utils.IsIpv6(mb.IpAddr) {
							sep = "."
						}
						fmtmbs = append(fmtmbs, map[string]interface{}{
							"name":    fmt.Sprintf("%s%s%d", mb.IpAddr, sep, mb.TargetPort),
							"address": mb.IpAddr,
						})
					}
					rlt["ltm/pool/"+name].(map[string]interface{})["members"] = fmtmbs

					// TODO: parse ARP resources for flannel type network.
				}
			}
		}
	}

	// irules
	name := strings.Join([]string{hr.Namespace, hr.Name}, ".")
	hostnameConditions := []string{}
	for _, hn := range hr.Spec.Hostnames {
		hostnameConditions = append(hostnameConditions, fmt.Sprintf(`[HTTP::host] matches "%s"`, hn))
	}
	hostnameCondition := strings.Join(hostnameConditions, " or ")
	if hostnameCondition == "" {
		hostnameCondition = "1 eq 1"
	}

	rules := []string{}
	for _, rl := range hr.Spec.Rules {
		ruleConditions := []string{}
		for _, match := range rl.Matches {
			matchConditions := []string{}
			if match.Path != nil {
				switch *match.Path.Type {
				case gatewayv1beta1.PathMatchPathPrefix:
					matchConditions = append(matchConditions, fmt.Sprintf(`[HTTP::path] starts_with "%s"`, *match.Path.Value))
				case gatewayv1beta1.PathMatchExact:
					matchConditions = append(matchConditions, fmt.Sprintf(`[HTTP::path] eq "%s"`, *match.Path.Value))
				case gatewayv1beta1.PathMatchRegularExpression:
					matchConditions = append(matchConditions, fmt.Sprintf(`[HTTP::path matches "%s"`, *match.Path.Value))
				}
			}
			if match.Headers != nil {
				return map[string]interface{}{}, fmt.Errorf("match type Headers not supported yet")
			}
			if match.Method != nil {
				return map[string]interface{}{}, fmt.Errorf("match type Method not supported yet")
			}
			if match.QueryParams != nil {
				return map[string]interface{}{}, fmt.Errorf("match type QueryParams not supported yet")
			}
			ruleConditions = append(ruleConditions, strings.Join(matchConditions, " and "))
		}
		ruleCondition := strings.Join(ruleConditions, " or ")
		if ruleCondition == "" {
			ruleCondition = "1 eq 1"
		}
		// TODO: only the last backendRef is used.
		var pool string
		for _, br := range rl.BackendRefs {
			ns := hr.Namespace
			if br.Namespace != nil {
				ns = string(*br.Namespace)
			}
			pool = strings.Join([]string{ns, string(br.Name)}, ".")

		}
		rules = append(rules, fmt.Sprintf(`	
				if { %s } {
					pool /cis-c-tenant/%s
				}
			`, ruleCondition, pool))
	}

	ruleObj := map[string]interface{}{
		"name": name,
		"apiAnonymous": fmt.Sprintf(`
			when HTTP_REQUEST {
				log local0. "request host: [HTTP::host], uri: [HTTP::uri], path: [HTTP::path]"
				if { %s } {
					%s
				}
			}
		`, hostnameCondition, strings.Join(rules, "\n")),
	}

	rlt["ltm/rule/"+name] = ruleObj

	return rlt, nil
}

func ParseGateway(gw *gatewayv1beta1.Gateway) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	if gw == nil {
		return map[string]interface{}{}, nil
	}

	rlt := map[string]interface{}{}

	hrs := ActiveSIGs.AttachedHTTPRoutes(gw)
	irules := map[string][]string{}
	for _, hr := range hrs {
		for _, pr := range hr.Spec.ParentRefs {
			ns := hr.Namespace
			if pr.Namespace != nil {
				ns = string(*pr.Namespace)
			}
			if pr.SectionName == nil {
				return map[string]interface{}{}, fmt.Errorf("sectionName of paraentRefs is nil, not supported yet")
			}
			listener := strings.Join([]string{ns, string(pr.Name), string(*pr.SectionName)}, ".")
			if _, ok := irules[listener]; !ok {
				irules[listener] = []string{}
			}
			irules[listener] = append(irules[listener], strings.Join([]string{hr.Namespace, hr.Name}, "."))
		}
	}
	for _, addr := range gw.Spec.Addresses {
		if *addr.Type == gatewayv1beta1.IPAddressType {
			ipaddr := addr.Value
			for _, listener := range gw.Spec.Listeners {
				var profiles []interface{}
				ipProtocol := ""
				switch listener.Protocol {
				case gatewayv1beta1.HTTPProtocolType:
					profiles = []interface{}{map[string]string{"name": "http"}}
					ipProtocol = "tcp"
				case gatewayv1beta1.TCPProtocolType:
					return map[string]interface{}{}, fmt.Errorf("unsupported ProtocolType: %s", listener.Protocol)
				case gatewayv1beta1.UDPProtocolType:
					return map[string]interface{}{}, fmt.Errorf("unsupported ProtocolType: %s", listener.Protocol)
				case gatewayv1beta1.TLSProtocolType:
					return map[string]interface{}{}, fmt.Errorf("unsupported ProtocolType: %s", listener.Protocol)
				}
				if ipProtocol == "" {
					return map[string]interface{}{}, fmt.Errorf("ipProtocol not set in %s case", listener.Protocol)
				}
				destination := fmt.Sprintf("%s:%d", ipaddr, listener.Port)
				if utils.IsIpv6(ipaddr) {
					destination = fmt.Sprintf("%s.%d", ipaddr, listener.Port)
				}
				name := strings.Join([]string{gw.Namespace, gw.Name, string(listener.Name)}, ".")

				rlt["ltm/virtual/"+name] = map[string]interface{}{
					"name":        name,
					"profiles":    profiles,
					"ipProtocol":  ipProtocol,
					"destination": destination,
					"sourceAddressTranslation": map[string]interface{}{
						"type": "automap",
					},
					"rules": []interface{}{},
				}
				if _, ok := irules[name]; ok {
					rlt["ltm/virtual/"+name].(map[string]interface{})["rules"] = irules[name]
				}
			}
		} else {
			return map[string]interface{}{}, fmt.Errorf("unsupported AddressType: %s", *addr.Type)
		}
	}

	return rlt, nil
}

func ParseRelated(gwObjs []*gatewayv1beta1.Gateway, hrObjs []*gatewayv1beta1.HTTPRoute) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	gwmap, hrmap := map[string]*gatewayv1beta1.Gateway{}, map[string]*gatewayv1beta1.HTTPRoute{}
	ActiveSIGs.GetRelatedObjs(gwObjs, hrObjs, &gwmap, &hrmap)

	rlt := map[string]interface{}{}
	for _, gw := range gwmap {
		if cfgs, err := ParseGateway(gw); err != nil {
			return map[string]interface{}{}, err
		} else {
			for k, v := range cfgs {
				rlt[k] = v
			}
		}
	}
	for _, hr := range hrmap {
		if cfgs, err := ParseHTTPRoute(hr); err != nil {
			return map[string]interface{}{}, err
		} else {
			for k, v := range cfgs {
				rlt[k] = v
			}
		}
	}

	return map[string]interface{}{
		"": rlt,
	}, nil
}
