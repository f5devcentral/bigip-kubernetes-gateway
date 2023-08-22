package pkg

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/k8s"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	v1 "k8s.io/api/core/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// func ParseGatewayRelatedForClass(className string, gwObjs []*gatewayv1beta1.Gateway) (map[string]interface{}, error) {
// 	defer utils.TimeItToPrometheus()()

// 	if gwc := ActiveSIGs.GetGatewayClass(className); gwc == nil ||
// 		gwc.Spec.ControllerName != gatewayv1beta1.GatewayController(ActiveSIGs.ControllerName) {
// 		return map[string]interface{}{}, nil
// 	}

// 	cgwObjs := []*gatewayv1beta1.Gateway{}
// 	for _, gw := range gwObjs {
// 		if gw.Spec.GatewayClassName == gatewayv1beta1.ObjectName(className) {
// 			cgwObjs = append(cgwObjs, gw)
// 		}
// 	}

// 	rlt := map[string]interface{}{}
// 	for _, gw := range cgwObjs {
// 		if err := parseGateway(gw, rlt); err != nil {
// 			return map[string]interface{}{}, err
// 		}
// 		hrs := ActiveSIGs.AttachedHTTPRoutes(gw)
// 		for _, hr := range hrs {
// 			if err := parseHTTPRoute(className, hr, rlt); err != nil {
// 				return map[string]interface{}{}, err
// 			}
// 		}
// 	}
// 	return map[string]interface{}{
// 		"": rlt,
// 	}, nil
// }

func ParseAllForClass(className string) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	var gwc *gatewayv1beta1.GatewayClass
	if gwc = ActiveSIGs.GetGatewayClass(className); gwc == nil ||
		gwc.Spec.ControllerName != gatewayv1beta1.GatewayController(ActiveSIGs.ControllerName) {
		return map[string]interface{}{}, nil
	}

	cgwObjs := ActiveSIGs.AttachedGateways(gwc)
	folder := "serviceMain"

	rlt := map[string]interface{}{}
	for _, gw := range cgwObjs {
		if err := parseGateway(gw, rlt); err != nil {
			return map[string]interface{}{}, err
		}
		hrs := ActiveSIGs.AttachedHTTPRoutes(gw)
		for _, hr := range hrs {
			if err := parseHTTPRoute(className, hr, rlt); err != nil {
				return map[string]interface{}{}, err
			}
		}
	}
	if len(rlt) == 0 {
		return nil, nil
	} else {
		return map[string]interface{}{
			folder: rlt,
		}, nil
	}

}

// ParseRelatedServices parse all refered services
func ParseClassRelatedServices(gwc []string) (map[string]interface{}, error) {

	svcs := []*v1.Service{}
	for _, c := range gwc {
		gc := ActiveSIGs.GetGatewayClass(c)
		svcs = append(svcs, ActiveSIGs.RelatedServices(gc)...)
	}

	keys := []string{}
	for _, svc := range svcs {
		keys = append(keys, utils.Keyname(svc.Namespace, svc.Name))
	}
	return ParseServices(keys)
}

func ParseServices(svcs []string) (map[string]interface{}, error) {
	rlts := map[string]interface{}{}
	for _, svc := range svcs {
		ns := strings.Split(svc, "/")[0]
		n := strings.Split(svc, "/")[1]

		if _, f := rlts[ns]; !f {
			rlts[ns] = map[string]interface{}{
				"serviceMain": map[string]interface{}{},
			}
		}
		// name := strings.Join([]string{ns, n}, ".")
		pool := map[string]interface{}{
			"class":    "Pool",
			"monitors": []string{"tcp"},
			"members":  []interface{}{},
		}
		if fmtmbs, err := parseMembersFrom(ns, n); err != nil {
			return nil, err
		} else {
			pool["members"] = fmtmbs
		}

		if mon, err := parseMonitorFrom(ns, n); err != nil {
			return nil, err
		} else {
			pool["monitors"] = mon
		}

		// if err := parseArpsFrom(ns, n, rlt); err != nil {
		// 	return rlt, err
		// }
		// if err := parseNodesFrom(ns, n, rlt); err != nil {
		// 	return rlt, err
		// }
		rlts[ns].(map[string]interface{})["serviceMain"].(map[string]interface{})["ltm/pool/"+n] = pool
	}

	return rlts, nil
}

func parseHTTPRoute(className string, hr *gatewayv1beta1.HTTPRoute, rlt map[string]interface{}) error {
	defer utils.TimeItToPrometheus()()

	if hr == nil {
		return nil
	}

	if err := parseiRulesFrom(className, hr, rlt); err != nil {
		return err
	}

	return nil
}

func parseGateway(gw *gatewayv1beta1.Gateway, rlt map[string]interface{}) error {
	defer utils.TimeItToPrometheus()()

	if gw == nil {
		return nil
	}
	irules := map[string][]string{}
	listeners := map[string]*gatewayv1beta1.Listener{}

	// listener mapping
	for i, listener := range gw.Spec.Listeners {
		vsname := gwListenerName(gw, &listener)
		listeners[vsname] = &gw.Spec.Listeners[i]
	}

	// irules mapping: when listener.Hostname is not nil
	for _, listener := range gw.Spec.Listeners {
		vsname := gwListenerName(gw, &listener)
		if listener.Hostname != nil {
			if _, ok := irules[vsname]; !ok {
				irules[vsname] = []string{}
			}
			irules[vsname] = append(irules[vsname], vsname)
			rule := map[string]interface{}{
				"class": "iRule",
				"iRule": fmt.Sprintf(`
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

	// irules mapping: for httproutes
	hrs := ActiveSIGs.AttachedHTTPRoutes(gw)
	for _, hr := range hrs {
		for _, pr := range hr.Spec.ParentRefs {
			ns := hr.Namespace
			if pr.Namespace != nil {
				ns = string(*pr.Namespace)
			}
			if pr.SectionName == nil {
				return fmt.Errorf("sectionName of paraentRefs is nil, not supported")
			}
			vsname := hrParentName(hr, &pr)
			if _, ok := irules[vsname]; !ok {
				irules[vsname] = []string{}
			}
			routetype := reflect.TypeOf(*hr).Name()
			if RouteMatches(ns, listeners[vsname], ActiveSIGs.GetNamespace(hr.Namespace), routetype) {
				irules[vsname] = append(irules[vsname], hrName(hr))
			}
		}
	}

	// clientssl if exists
	scrtmap, err := ActiveSIGs.AttachedSecrets(gw)
	if err != nil {
		return err
	}
	for k, scrts := range scrtmap {
		// for i, scrt := range scrts {
		parseSecrets(k, scrts, rlt)
		// }
	}

	// virtual
	for i, addr := range gw.Spec.Addresses {
		if *addr.Type == gatewayv1beta1.IPAddressType {
			ipaddr := addr.Value
			for _, listener := range gw.Spec.Listeners {
				virtual := map[string]interface{}{}

				lsname := gwListenerName(gw, &listener)
				vrname := fmt.Sprintf("%s.%d", gwListenerName(gw, &listener), i)
				switch listener.Protocol {
				case gatewayv1beta1.HTTPProtocolType:
					virtual["profileHTTP"] = "basic"
					virtual["class"] = "Service_HTTP"
				case gatewayv1beta1.HTTPSProtocolType:
					virtual["class"] = "Service_HTTPS"
					virtual["profileHTTP"] = "basic"
					virtual["serverTLS"] = lsname
				case gatewayv1beta1.TCPProtocolType:
					return fmt.Errorf("unsupported ProtocolType: %s", listener.Protocol)
				case gatewayv1beta1.UDPProtocolType:
					return fmt.Errorf("unsupported ProtocolType: %s", listener.Protocol)
				case gatewayv1beta1.TLSProtocolType:
					return fmt.Errorf("unsupported ProtocolType: %s", listener.Protocol)
				}

				virtual["virtualAddresses"] = []string{ipaddr}
				virtual["virtualPort"] = listener.Port
				virtual["iRules"] = irules[lsname]
				virtual["snat"] = "auto"
				rlt["ltm/virtual/"+vrname] = virtual
			}
		} else {
			return fmt.Errorf("unsupported AddressType: %s", *addr.Type)
		}
	}

	return nil
}

// TODO: find the way to set monitor
func parseMonitorFrom(svcNamespace, svcName string) (interface{}, error) {
	return []string{"tcp"}, nil
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
				fmtmbs = append(fmtmbs, map[string]interface{}{
					"servicePort":     mb.TargetPort,
					"serverAddresses": []string{mb.IpAddr},
				})
			}
			return fmtmbs, nil
		}
	} else {
		return []interface{}{}, nil
	}
}

// func parseArpsFrom(svcNamespace, svcName string, rlt map[string]interface{}) error {
// 	svc := ActiveSIGs.GetService(utils.Keyname(svcNamespace, svcName))
// 	eps := ActiveSIGs.GetEndpoints(utils.Keyname(svcNamespace, svcName))
// 	if svc != nil && eps != nil {
// 		if mbs, err := k8s.FormatMembersFromServiceEndpoints(svc, eps); err != nil {
// 			return err
// 		} else {
// 			prefix := "k8s-"
// 			for _, mb := range mbs {
// 				if mb.MacAddr != "" {
// 					rlt["net/arp/"+prefix+mb.IpAddr] = map[string]interface{}{
// 						"name":       prefix + mb.IpAddr,
// 						"ipAddress":  mb.IpAddr,
// 						"macAddress": mb.MacAddr,
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }

// func parseNodesFrom(svcNamespace, svcName string, rlt map[string]interface{}) error {
// 	svc := ActiveSIGs.GetService(utils.Keyname(svcNamespace, svcName))
// 	eps := ActiveSIGs.GetEndpoints(utils.Keyname(svcNamespace, svcName))
// 	if svc != nil && eps != nil {
// 		if mbs, err := k8s.FormatMembersFromServiceEndpoints(svc, eps); err != nil {
// 			return err
// 		} else {
// 			for _, mb := range mbs {
// 				rlt["ltm/node/"+mb.IpAddr] = map[string]interface{}{
// 					"name":    mb.IpAddr,
// 					"address": mb.IpAddr,
// 					"monitor": "default",
// 					"session": "user-enabled",
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }

func parseSecrets(lsname string, scrts []*v1.Secret, rlt map[string]interface{}) {
	certs := []interface{}{}
	for _, scrt := range scrts {
		crtContent := string(scrt.Data[v1.TLSCertKey])
		keyContent := string(scrt.Data[v1.TLSPrivateKeyKey])

		name := tlsName(scrt)

		rlt["sys/file/certificate/"+name] = map[string]interface{}{
			"class":       "Certificate",
			"certificate": crtContent,
			"privateKey":  keyContent,
		}
		certs = append(certs, map[string]interface{}{
			"certificate": name,
			// "sniDefault":  i == 0,
		})
	}

	if len(certs) > 0 {
		rlt["ltm/profile/client-ssl/"+lsname] = map[string]interface{}{
			"class":        "TLS_Server",
			"certificates": certs,
		}
	}
}
