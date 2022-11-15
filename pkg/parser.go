package pkg

import (
	"fmt"
	"strings"

	"gitee.com/zongzw/bigip-kubernetes-gateway/k8s"
	"gitee.com/zongzw/f5-bigip-rest/utils"
	v1 "k8s.io/api/core/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func ParseHTTPRoute(hr *gatewayv1beta1.HTTPRoute) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	if hr == nil {
		return map[string]interface{}{}, nil
	}

	rlt := map[string]interface{}{}
	if err := parsePoolsFrom(hr, rlt); err != nil {
		return map[string]interface{}{}, err
	}

	if err := parseiRulesFrom(hr, rlt); err != nil {
		return map[string]interface{}{}, err
	}

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

func ParseRelated(gwObjs []*gatewayv1beta1.Gateway, hrObjs []*gatewayv1beta1.HTTPRoute, svcObjs []*v1.Service) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	gwmap, hrmap, svcmap := map[string]*gatewayv1beta1.Gateway{}, map[string]*gatewayv1beta1.HTTPRoute{}, map[string]*v1.Service{}
	ActiveSIGs.GetRelatedObjs(gwObjs, hrObjs, svcObjs, &gwmap, &hrmap, &svcmap)

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

func parsePoolsFrom(hr *gatewayv1beta1.HTTPRoute, rlt map[string]interface{}) error {

	creatPool := func(ns, n string, rlt map[string]interface{}) error {
		name := strings.Join([]string{ns, n}, ".")
		rlt["ltm/pool/"+name] = map[string]interface{}{
			"name":    name,
			"monitor": "min 1 of tcp",
			"members": []interface{}{},

			// "minActiveMembers": 0,
			// TODO: there's at least one field for PATCH a pool. or we may need to fix that
			// {"code":400,"message":"transaction failed:one or more properties must be specified","errorStack":[],"apiError":2}
		}
		if fmtmbs, err := parseMembersFrom(ns, n); err != nil {
			return err
		} else {
			rlt["ltm/pool/"+name].(map[string]interface{})["members"] = fmtmbs
		}

		if mon, err := parseMonitorFrom(ns, n); err != nil {
			return err
		} else {
			rlt["ltm/pool/"+name].(map[string]interface{})["monitor"] = mon
		}

		if err := parseArpsFrom(ns, n, rlt); err != nil {
			return err
		}
		if err := parseNodesFrom(ns, n, rlt); err != nil {
			return err
		}
		return nil
	}

	for _, rl := range hr.Spec.Rules {
		for _, br := range rl.BackendRefs {
			ns := hr.Namespace
			if br.Namespace != nil {
				ns = string(*br.Namespace)
			}
			if err := creatPool(ns, string(br.Name), rlt); err != nil {
				return err
			}
		}
	}

	// pools from ExtensionRef as well.
	for _, rl := range hr.Spec.Rules {
		for _, fl := range rl.Filters {
			if fl.Type == gatewayv1beta1.HTTPRouteFilterExtensionRef && fl.ExtensionRef != nil {
				er := fl.ExtensionRef
				if er.Group != "v1" || er.Kind != "Service" {
					return fmt.Errorf("resource %s of '%s' not supported", er.Name, utils.Keyname(string(er.Group), string(er.Kind)))
				} else {
					if err := creatPool(hr.Namespace, string(er.Name), rlt); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// TODO: find the way to set monitor
func parseMonitorFrom(svcNamespace, svcName string) (string, error) {
	return "min 1 of tcp", nil
}

func parseMembersFrom(svcNamespace, svcName string) ([]interface{}, error) {
	svc := ActiveSIGs.GetService(utils.Keyname(svcNamespace, svcName))
	eps := ActiveSIGs.GetEndpoints(utils.Keyname(svcNamespace, svcName))
	if svc != nil && eps != nil {
		if mbs, err := k8s.FormatMembersFromServiceEndpoints(svc, eps); err != nil {
			return []interface{}{}, err
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
			return fmtmbs, nil
		}
	} else {
		return []interface{}{}, nil
	}
}

func parseArpsFrom(svcNamespace, svcName string, rlt map[string]interface{}) error {
	svc := ActiveSIGs.GetService(utils.Keyname(svcNamespace, svcName))
	eps := ActiveSIGs.GetEndpoints(utils.Keyname(svcNamespace, svcName))
	if svc != nil && eps != nil {
		if mbs, err := k8s.FormatMembersFromServiceEndpoints(svc, eps); err != nil {
			return err
		} else {
			prefix := "k8s-"
			for _, mb := range mbs {
				if mb.MacAddr != "" {
					rlt["net/arp/"+prefix+mb.IpAddr] = map[string]interface{}{
						"name":       prefix + mb.IpAddr,
						"ipAddress":  mb.IpAddr,
						"macAddress": mb.MacAddr,
					}
				}
			}
		}
	}
	return nil
}

func parseNodesFrom(svcNamespace, svcName string, rlt map[string]interface{}) error {
	svc := ActiveSIGs.GetService(utils.Keyname(svcNamespace, svcName))
	eps := ActiveSIGs.GetEndpoints(utils.Keyname(svcNamespace, svcName))
	if svc != nil && eps != nil {
		if mbs, err := k8s.FormatMembersFromServiceEndpoints(svc, eps); err != nil {
			return err
		} else {
			for _, mb := range mbs {
				if mb.MacAddr != "" {
					rlt["ltm/node/"+mb.IpAddr] = map[string]interface{}{
						"name":    mb.IpAddr,
						"address": mb.IpAddr,
						"monitor": "default",
						"session": "user-enabled",
					}
				}
			}
		}
	}
	return nil
}

func parseiRulesFrom(hr *gatewayv1beta1.HTTPRoute, rlt map[string]interface{}) error {
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
		var pool string = ""
		ruleConditions := []string{}
		filterActions := []string{}

		for _, match := range rl.Matches {
			matchConditions := []string{}
			if match.Path != nil {
				matchType := gatewayv1beta1.PathMatchPathPrefix
				if match.Path.Type != nil {
					matchType = *match.Path.Type
				}
				switch matchType {
				case gatewayv1beta1.PathMatchPathPrefix:
					matchConditions = append(matchConditions, fmt.Sprintf(`[HTTP::path] starts_with "%s"`, *match.Path.Value))
				case gatewayv1beta1.PathMatchExact:
					matchConditions = append(matchConditions, fmt.Sprintf(`[HTTP::path] eq "%s"`, *match.Path.Value))
				case gatewayv1beta1.PathMatchRegularExpression:
					matchConditions = append(matchConditions, fmt.Sprintf(`[HTTP::path matches "%s"`, *match.Path.Value))
				}
			}
			if match.Headers != nil {
				for _, header := range match.Headers {
					matchType := gatewayv1beta1.HeaderMatchExact
					if header.Type != nil {
						matchType = *header.Type
					}
					switch matchType {
					case gatewayv1beta1.HeaderMatchExact:
						matchConditions = append(matchConditions, fmt.Sprintf(`[HTTP::header "%s"] eq "%s"`, header.Name, header.Value))
					case gatewayv1beta1.HeaderMatchRegularExpression:
						matchConditions = append(matchConditions, fmt.Sprintf(`[HTTP::header "%s"] matches "%s"`, header.Name, header.Value))
					}
				}
			}
			if match.Method != nil {
				matchConditions = append(matchConditions, fmt.Sprintf(`[HTTP::method] eq "%s"`, *match.Method))
			}
			if match.QueryParams != nil {
				for _, queryParam := range match.QueryParams {
					matchType := gatewayv1beta1.QueryParamMatchExact
					if queryParam.Type != nil {
						matchType = *queryParam.Type
					}
					switch matchType {
					case gatewayv1beta1.QueryParamMatchExact:
						matchConditions = append(matchConditions, fmt.Sprintf(`[URI::query [HTTP::uri] "%s"] eq "%s"`, queryParam.Name, queryParam.Value))
					case gatewayv1beta1.QueryParamMatchRegularExpression:
						matchConditions = append(matchConditions, fmt.Sprintf(`[URI::query [HTTP::uri] "%s"] matches "%s"`, queryParam.Name, queryParam.Value))
					}
				}
			}
			ruleConditions = append(ruleConditions, strings.Join(matchConditions, " and "))
		}
		ruleCondition := strings.Join(ruleConditions, " or ")
		if ruleCondition == "" {
			ruleCondition = "1 eq 1"
		}

		// filters
		for _, filter := range rl.Filters {
			switch filter.Type {
			case gatewayv1beta1.HTTPRouteFilterRequestHeaderModifier:
				if filter.RequestHeaderModifier != nil {
					for _, mdr := range filter.RequestHeaderModifier.Add {
						filterActions = append(filterActions, fmt.Sprintf("HTTP::header insert %s %s", mdr.Name, mdr.Value))
					}
					for _, mdr := range filter.RequestHeaderModifier.Remove {
						filterActions = append(filterActions, fmt.Sprintf("HTTP::header remove %s", mdr))
					}
					for _, mdr := range filter.RequestHeaderModifier.Set {
						filterActions = append(filterActions, fmt.Sprintf("HTTP::header replace %s %s", mdr.Name, mdr.Value))
					}
				}
			case gatewayv1beta1.HTTPRouteFilterRequestMirror:
				// filter.RequestMirror.BackendRef -> vs mirror?
				return fmt.Errorf("filter type '%s' not supported", gatewayv1beta1.HTTPRouteFilterRequestMirror)
			case gatewayv1beta1.HTTPRouteFilterRequestRedirect:
				if rr := filter.RequestRedirect; rr != nil {
					setScheme := `set rscheme "http"`
					if rr.Scheme != nil {
						setScheme = fmt.Sprintf(`set rscheme "%s"`, *rr.Scheme)
					}
					setHostName := `set rhostname "[HTTP::host]"`
					if rr.Hostname != nil {
						setHostName = fmt.Sprintf(`set rhostname "%s"`, *rr.Hostname)
					}

					// experimental .. definition is not clear yet.
					setUri := `set ruri "[HTTP::uri]"`
					if rr.Path != nil && rr.Path.ReplaceFullPath != nil {
						setUri = fmt.Sprintf(`set ruri "%s"`, *rr.Path.ReplaceFullPath)
					}

					setPort := `set rport [TCP::local_port]`
					if rr.Port != nil {
						setPort = fmt.Sprintf(`set rport %d`, *rr.Port)
					}

					if rr.StatusCode != nil {
						if *rr.StatusCode != 301 && *rr.StatusCode != 302 {
							return fmt.Errorf("invalid status %d for request redirect", *rr.StatusCode)
						}
					}
					filterActions = append(filterActions, fmt.Sprintf(`
						%s
						%s
						%s
						%s
						set url $rscheme://$rhostname:$rport$ruri
						log local0. "request redirect to $url"
						HTTP::respond %d Location $url
					`, setScheme, setHostName, setUri, setPort, *rr.StatusCode))
				}
			// <gateway:experimental>
			case gatewayv1beta1.HTTPRouteFilterURLRewrite:
				if ur := filter.URLRewrite; ur != nil {
					setHostname := `set rhostname [HTTP::host]`
					if ur.Hostname != nil {
						setHostname = fmt.Sprintf(`set rhostname "%s"`, *ur.Hostname)
					}
					// experimental .. definition is not clear yet.
					setPath := `set rpath [HTTP::path]`
					if ur.Path != nil && ur.Path.ReplaceFullPath != nil {
						setPath = fmt.Sprintf(`set rpath "%s"`, *ur.Path.ReplaceFullPath)
					}
					filterActions = append(filterActions, fmt.Sprintf(`
						%s
						%s
						[HTTP::header replace Host $rhostname]
						HTTP::uri $rpath
					`, setHostname, setPath))
				}
			case gatewayv1beta1.HTTPRouteFilterExtensionRef:
				if er := filter.ExtensionRef; er != nil {
					pool := fmt.Sprintf("%s.%s", hr.Namespace, er.Name)
					filterActions = append(filterActions, fmt.Sprintf("pool /cis-c-tenant/%s", pool))
				}
			}
		}
		filterAction := strings.Join(filterActions, "\n")

		// TODO: only the last backendRef is used.
		for _, br := range rl.BackendRefs {
			ns := hr.Namespace
			if br.Namespace != nil {
				ns = string(*br.Namespace)
			}
			pn := strings.Join([]string{ns, string(br.Name)}, ".")
			pool = fmt.Sprintf("pool /cis-c-tenant/%s", pn)
		}
		rules = append(rules, fmt.Sprintf(`	
			if { %s } {
				%s
				%s
			}
		`, ruleCondition, filterAction, pool))
	}

	ruleObj := map[string]interface{}{
		"name": name,
		"apiAnonymous": fmt.Sprintf(`
		when HTTP_REQUEST {
			log local0. "request host: [HTTP::host], uri: [HTTP::uri], path: [HTTP::path], method: [HTTP::method]"
			log local0. "headers: [HTTP::header names]"
			foreach header [HTTP::header names] {
				log local0. "$header: [HTTP::header value $header]"
			}
			log local0. "queryparams: [HTTP::query]"
			if { %s } {
				%s
			}
		}
	`, hostnameCondition, strings.Join(rules, "\n")),
	}

	rlt["ltm/rule/"+name] = ruleObj
	return nil
}
