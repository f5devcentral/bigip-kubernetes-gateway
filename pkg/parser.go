package pkg

import (
	"fmt"
	"reflect"
	"strings"

	"gitee.com/zongzw/bigip-kubernetes-gateway/k8s"
	"gitee.com/zongzw/f5-bigip-rest/utils"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func ParseGatewayRelatedForClass(className string, gwObjs []*gatewayv1beta1.Gateway) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	if ActiveSIGs.GetGatewayClass(className) == nil {
		return map[string]interface{}{}, nil
	}

	cgwObjs := []*gatewayv1beta1.Gateway{}
	for _, gw := range gwObjs {
		if gw.Spec.GatewayClassName == gatewayv1beta1.ObjectName(className) {
			cgwObjs = append(cgwObjs, gw)
		}
	}

	rlt := map[string]interface{}{}
	for _, gw := range cgwObjs {
		if cfgs, err := parseGateway(gw); err != nil {
			return map[string]interface{}{}, err
		} else {
			for k, v := range cfgs {
				rlt[k] = v
			}
		}
		hrs := ActiveSIGs.AttachedHTTPRoutes(gw)
		for _, hr := range hrs {
			if cfgs, err := parseHTTPRoute(className, hr); err != nil {
				return map[string]interface{}{}, err
			} else {
				for k, v := range cfgs {
					rlt[k] = v
				}
			}
		}
	}
	return map[string]interface{}{
		"": rlt,
	}, nil
}

// ParseServicesRelatedForAll parse all refered services
func ParseServicesRelatedForAll() (map[string]interface{}, error) {

	// all services that are referenced but may not exist
	svcs := ActiveSIGs.AllAttachedServiceKeys()

	return ParseReferedServiceKeys(svcs)
}

func ParseReferedServiceKeys(svcs []string) (map[string]interface{}, error) {
	rlt := map[string]interface{}{}
	for _, svc := range svcs {

		ns := strings.Split(svc, "/")[0]
		n := strings.Split(svc, "/")[1]

		name := strings.Join([]string{ns, n}, ".")
		rlt["ltm/pool/"+name] = map[string]interface{}{
			"name":    name,
			"monitor": "min 1 of tcp",
			"members": []interface{}{},
		}
		if fmtmbs, err := parseMembersFrom(ns, n); err != nil {
			return rlt, err
		} else {
			rlt["ltm/pool/"+name].(map[string]interface{})["members"] = fmtmbs
		}

		if mon, err := parseMonitorFrom(ns, n); err != nil {
			return rlt, err
		} else {
			rlt["ltm/pool/"+name].(map[string]interface{})["monitor"] = mon
		}

		if err := parseArpsFrom(ns, n, rlt); err != nil {
			return rlt, err
		}
		if err := parseNodesFrom(ns, n, rlt); err != nil {
			return rlt, err
		}
	}

	return map[string]interface{}{
		"": rlt,
	}, nil
}

func parseHTTPRoute(className string, hr *gatewayv1beta1.HTTPRoute) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	if hr == nil {
		return map[string]interface{}{}, nil
	}

	rlt := map[string]interface{}{}

	if err := parseiRulesFrom(className, hr, rlt); err != nil {
		return map[string]interface{}{}, err
	}

	return rlt, nil
}

func parseGateway(gw *gatewayv1beta1.Gateway) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	if gw == nil {
		return map[string]interface{}{}, nil
	}

	rlt := map[string]interface{}{}
	irules := map[string][]string{}
	listeners := map[string]*gatewayv1beta1.Listener{}

	for i, listener := range gw.Spec.Listeners {
		vsname := gwListenerName(gw, &listener)
		listeners[vsname] = &gw.Spec.Listeners[i]
		if listener.Hostname != nil {
			if _, ok := irules[vsname]; !ok {
				irules[vsname] = []string{}
			}
			irules[vsname] = append(irules[vsname], vsname)
			rule := map[string]interface{}{
				"name": vsname,
				"apiAnonymous": fmt.Sprintf(`
					when HTTP_REQUEST {
						if { not ([HTTP::host] matches "%s") } {
							event HTTP_REQUEST disable
						}
					}
				`, *listener.Hostname),
			}
			rlt["ltm/rule/"+vsname] = rule
		}
	}

	hrs := ActiveSIGs.AttachedHTTPRoutes(gw)
	for _, hr := range hrs {
		for _, pr := range hr.Spec.ParentRefs {
			ns := hr.Namespace
			if pr.Namespace != nil {
				ns = string(*pr.Namespace)
			}
			if pr.SectionName == nil {
				return map[string]interface{}{}, fmt.Errorf("sectionName of paraentRefs is nil, not supported")
			}
			vsname := hrParentName(hr, &pr)
			if _, ok := irules[vsname]; !ok {
				irules[vsname] = []string{}
			}
			routetype := reflect.TypeOf(*hr).Name()
			if routeMatches(ns, listeners[vsname], ActiveSIGs.GetNamespace(hr.Namespace), routetype) {
				irules[vsname] = append(irules[vsname], hrName(hr))
			}
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
				name := gwListenerName(gw, &listener)

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

func parseiRulesFrom(className string, hr *gatewayv1beta1.HTTPRoute, rlt map[string]interface{}) error {
	name := hrName(hr)

	// hostnames
	hostnameConditions := []string{}
	for _, hn := range hr.Spec.Hostnames {
		hostnameConditions = append(hostnameConditions, fmt.Sprintf(`[HTTP::host] matches "%s"`, hn))
	}
	hostnameCondition := strings.Join(hostnameConditions, " or ")
	if hostnameCondition == "" {
		hostnameCondition = "1 eq 1"
	}

	respRules := ""
	reqRules := []string{}
	ruleInits := []string{}
	for i, rl := range hr.Spec.Rules {
		ruleConditions := []string{}
		//filterActions := []string{}
		reqFilterActions := []string{}
		respFilterActions := []string{}
		poolWeights := []string{}

		// matches
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
						reqFilterActions = append(reqFilterActions, fmt.Sprintf("HTTP::header insert %s %s", mdr.Name, mdr.Value))
					}
					for _, mdr := range filter.RequestHeaderModifier.Remove {
						reqFilterActions = append(reqFilterActions, fmt.Sprintf("HTTP::header remove %s", mdr))
					}
					for _, mdr := range filter.RequestHeaderModifier.Set {
						reqFilterActions = append(reqFilterActions, fmt.Sprintf("HTTP::header replace %s %s", mdr.Name, mdr.Value))
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
					reqFilterActions = append(reqFilterActions, fmt.Sprintf(`
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
			// case gatewayv1beta1.HTTPRouteFilterURLRewrite:
			// 	if ur := filter.URLRewrite; ur != nil {
			// 		setHostname := `set rhostname [HTTP::host]`
			// 		if ur.Hostname != nil {
			// 			setHostname = fmt.Sprintf(`set rhostname "%s"`, *ur.Hostname)
			// 		}
			// 		// experimental .. definition is not clear yet.
			// 		setPath := `set rpath [HTTP::path]`
			// 		if ur.Path != nil && ur.Path.ReplaceFullPath != nil {
			// 			setPath = fmt.Sprintf(`set rpath "%s"`, *ur.Path.ReplaceFullPath)
			// 		}
			// 		filterActions = append(filterActions, fmt.Sprintf(`
			// 			%s
			// 			%s
			// 			[HTTP::header replace Host $rhostname]
			// 			HTTP::uri $rpath
			// 		`, setHostname, setPath))
			// 	}
			case gatewayv1beta1.HTTPRouteFilterExtensionRef:
				if er := filter.ExtensionRef; er != nil {
					pool := fmt.Sprintf("%s.%s", hr.Namespace, er.Name)
					reqFilterActions = append(reqFilterActions, fmt.Sprintf("pool /%s/%s", "cis-c-tenant", pool))
				}
			case gatewayv1beta1.HTTPRouteFilterResponseHeaderModifier:
				if filter.ResponseHeaderModifier != nil {
					for _, mdr := range filter.ResponseHeaderModifier.Add {
						respFilterActions = append(respFilterActions, fmt.Sprintf("HTTP::header insert %s %s", mdr.Name, mdr.Value))
					}
					for _, mdr := range filter.ResponseHeaderModifier.Remove {
						respFilterActions = append(respFilterActions, fmt.Sprintf("HTTP::header remove %s", mdr))
					}
					for _, mdr := range filter.ResponseHeaderModifier.Set {
						respFilterActions = append(respFilterActions, fmt.Sprintf("HTTP::header replace %s %s", mdr.Name, mdr.Value))
					}
				}
			}
		}
		reqFilterAction := strings.Join(reqFilterActions, "\n")

		for _, br := range rl.BackendRefs {
			ns := hr.Namespace
			if br.Namespace != nil {
				ns = string(*br.Namespace)
			}
			pn := strings.Join([]string{ns, string(br.Name)}, ".")
			pool := fmt.Sprintf("/%s/%s", "cis-c-tenant", pn)
			weight := 1
			if br.Weight != nil {
				weight = int(*br.Weight)
			}
			poolWeights = append(poolWeights, fmt.Sprintf("%s %d", pool, weight))
		}

		namedi := strings.ReplaceAll(fmt.Sprintf("%s_%d", name, i), ".", "_")
		namedi = strings.ReplaceAll(namedi, "-", "_")
		ruleInit := fmt.Sprintf(`

			array unset weights *
			array unset static::pools_%s *
			set index 0
			
			array set weights { %s }
			foreach name [array names weights] {
				for { set i 0 }  { $i < $weights($name) }  { incr i } {
					set static::pools_%s($index) $name
					incr index
				}
			}
			set static::pools_%s_size [array size static::pools_%s]
		`, namedi, strings.Join(poolWeights, " "), namedi, namedi, namedi)

		reqRules = append(reqRules, fmt.Sprintf(`	
			if { %s } {
				%s
				%s
			}
		`, ruleCondition, reqFilterAction, fmt.Sprintf(`
			set pool $static::pools_%s([expr {int(rand()*$static::pools_%s_size)}])
			pool $pool
			return
		`, namedi, namedi)))

		respRules = strings.Join(respFilterActions, "\n")

		ruleInits = append(ruleInits, ruleInit)
	}

	ruleObj := map[string]interface{}{
		"name": name,
		"apiAnonymous": fmt.Sprintf(
			`
			when RULE_INIT {
				%s
			}
			when HTTP_REQUEST {
				# log local0. "request host: [HTTP::host], uri: [HTTP::uri], path: [HTTP::path], method: [HTTP::method]"
				# log local0. "headers: [HTTP::header names]"
				# foreach header [HTTP::header names] {
				#	 log local0. "$header: [HTTP::header value $header]"
				# }
				# log local0. "queryparams: [HTTP::query]"
				if { %s } {
					%s
				}
			}
			when HTTP_RESPONSE {
				%s
			}
			`,
			strings.Join(ruleInits, "\n"),
			hostnameCondition, strings.Join(reqRules, "\n"),
			respRules),
	}

	rlt["ltm/rule/"+name] = ruleObj
	return nil
}

func parseNeighsFrom(routerName, localAs, remoteAs string, addresses []string) (map[string]interface{}, error) {
	rlt := map[string]interface{}{}

	name := strings.Join([]string{"Common", routerName}, ".")
	rlt["net/routing/bgp/"+name] = map[string]interface{}{
		"name":     name,
		"localAs":  localAs,
		"neighbor": []interface{}{},
	}

	fmtneigs := []interface{}{}
	for _, address := range addresses {
		fmtneigs = append(fmtneigs, map[string]interface{}{
			"name":     address,
			"remoteAs": remoteAs,
		})
	}

	rlt["net/routing/bgp/"+name].(map[string]interface{})["neighbor"] = fmtneigs

	return rlt, nil
}

func parseFdbsFrom(tunnelName string, iPToMac map[string]string) (map[string]interface{}, error) {
	rlt := map[string]interface{}{}

	rlt["net/fdb/tunnel/"+tunnelName] = map[string]interface{}{
		"records": []interface{}{},
	}

	fmtrecords := []interface{}{}
	for ip, mac := range iPToMac {
		fmtrecords = append(fmtrecords, map[string]string{
			"name":     mac,
			"endpoint": ip,
		})
	}

	rlt["net/fdb/tunnel/"+tunnelName].(map[string]interface{})["records"] = fmtrecords

	return rlt, nil
}

func ParseNodeConfigs(bc *BIGIPConfig) (map[string]interface{}, error) {
	cfgs := map[string]interface{}{}

	if bc.Calico != nil {
		nIpAddresses := k8s.NodeCache.AllIpAddresses()
		if ccfgs, err := parseNeighsFrom("gwcBGP", bc.Calico.LocalAS, bc.Calico.RemoteAS, nIpAddresses); err != nil {
			return map[string]interface{}{}, err
		} else {
			for k, v := range ccfgs {
				cfgs[k] = v
			}
		}
	}

	if bc.Flannel != nil {
		nIpToMacV4, _ := k8s.NodeCache.AllIpToMac()
		for _, tunnel := range bc.Flannel.Tunnels {
			if fcfgs, err := parseFdbsFrom(tunnel.Name, nIpToMacV4); err != nil {
				return map[string]interface{}{}, err
			} else {
				for k, v := range fcfgs {
					cfgs[k] = v
				}
			}
		}
	}

	return map[string]interface{}{
		"": cfgs,
	}, nil
}
