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

func ParseGatewayRelatedForClass(className string, gwObjs []*gatewayv1beta1.Gateway) (map[string]interface{}, error) {
	defer utils.TimeItToPrometheus()()

	if gwc := ActiveSIGs.GetGatewayClass(className); gwc == nil ||
		gwc.Spec.ControllerName != gatewayv1beta1.GatewayController(ActiveSIGs.ControllerName) {
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
			if RouteMatches(ns, listeners[vsname], ActiveSIGs.GetNamespace(hr.Namespace), routetype) {
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
				case gatewayv1beta1.HTTPSProtocolType:
					profiles = []interface{}{map[string]string{"name": "http"}}
					if listener.TLS == nil {
						return map[string]interface{}{}, fmt.Errorf("listener.TLS must be set for protocol %s", listener.Protocol)
					} else {
						if listener.TLS.Mode == nil || *listener.TLS.Mode == gatewayv1beta1.TLSModeTerminate {
							ssl_profiles := parseSSL(&listener, rlt, gw)
							for _, ssl := range ssl_profiles {
								profiles = append(profiles, map[string]string{"name": ssl})
							}
						}
					}
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
				rlt["ltm/virtual-address/"+ipaddr] = map[string]interface{}{
					"name":               ipaddr, // must be set: 403, {"code":403,"message":"Operation is not supported on component /ltm/virtual-address."
					"address":            ipaddr,
					"mask":               "255.255.255.255",
					"icmpEcho":           "enabled",
					"routeAdvertisement": "disabled",
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
				rlt["ltm/node/"+mb.IpAddr] = map[string]interface{}{
					"name":    mb.IpAddr,
					"address": mb.IpAddr,
					"monitor": "default",
					"session": "user-enabled",
				}
			}
		}
	}
	return nil
}

func parseSSL(ls *gatewayv1beta1.Listener, rlt map[string]any, gw *gatewayv1beta1.Gateway) []string {
	certRefs := ls.TLS.CertificateRefs
	var profileNames []string

	ns := gw.Namespace
	for i, cert := range certRefs {
		if cert.Namespace != nil {
			ns = string(*cert.Namespace)
		}
		nsn := utils.Keyname(ns, string(cert.Name))
		scrt := ActiveSIGs.GetSecret(nsn)

		/*
			TODO: what if existed gateway (old/new) (yaml) uses some tls secrets
			are deleted by user? we need some ways to tell user the gateway yaml
			is not up to date or wrong.

			1. It prevents user from misconfiguring the gateway.
			2. It prevents bigip from dirty configuration.

			Maybe we needs status or webhook to solve this problem.

			For now, we just IGNORE the problem, and leave some dirty tls
			configurations on BigIP.
		*/
		if scrt == nil || scrt.Type != v1.SecretTypeTLS {
			continue
		}
		if !ActiveSIGs.CanRefer(gw, scrt) {
			continue
		}
		crtContent := string(scrt.Data[v1.TLSCertKey])
		keyContent := string(scrt.Data[v1.TLSPrivateKeyKey])

		crtName := ns + "_" + string(cert.Name) + ".crt"
		keyName := ns + "_" + string(cert.Name) + ".key"

		rlt["shared/file-transfer/uploads/"+crtName] = map[string]any{
			"content": crtContent,
		}
		rlt["shared/file-transfer/uploads/"+keyName] = map[string]any{
			"content": keyContent,
		}

		rlt["sys/file/ssl-cert/"+crtName] = map[string]any{
			"name":       crtName,
			"sourcePath": "file:/var/config/rest/downloads/" + crtName,
		}
		rlt["sys/file/ssl-key/"+keyName] = map[string]any{
			"name":       keyName,
			"sourcePath": "file:/var/config/rest/downloads/" + keyName,
			"passphrase": "",
		}

		profileName := ns + "_" + string(cert.Name)

		rlt["ltm/profile/client-ssl/"+profileName] = map[string]any{
			"name":       profileName,
			"cert":       crtName,
			"key":        keyName,
			"sniDefault": i == 0,
		}

		profileNames = append(profileNames, profileName)
	}
	return profileNames
}
