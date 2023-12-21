package pkg

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
)

var iruleTemplate *template.Template

// issues: https://github.com/golang/go/issues/46056
// I have to move the templates to this directory.

//go:embed irule_templates/*.tmpl
var tmpls embed.FS

func init() {

	funcs := template.FuncMap{
		"orHosts":          orHostnames,
		"parseMatches":     parseiRuleMatches,
		"parseReqFilters":  parseiRuleReqFilters,
		"parseRespFilters": parseiRuleRespFilters,
		"parsePoolweight":  parsePoolweight,
	}

	iruleTemplate = template.Must(template.New("").Funcs(funcs).ParseFS(tmpls, "irule_templates/*.tmpl"))
}

func orHostnames(prefix string, hostnames []gatewayapi.Hostname) string {
	conditions := []string{}
	for _, hst := range hostnames {
		conditions = append(conditions, fmt.Sprint(prefix, ` "`, hst, `"`))
	}
	ret := strings.Join(conditions, " or ")
	return ret
}

func parseiRuleMatches(matches []gatewayapi.HTTPRouteMatch) string {
	matchConditions := []string{}
	for _, match := range matches {
		singleMatch := []string{}

		if match.Path != nil {
			matchType := gatewayapi.PathMatchPathPrefix
			if match.Path.Type != nil {
				matchType = *match.Path.Type
			}
			switch matchType {
			case gatewayapi.PathMatchPathPrefix:
				singleMatch = append(singleMatch, fmt.Sprintf(`[HTTP::path] starts_with "%s"`, *match.Path.Value))
			case gatewayapi.PathMatchExact:
				singleMatch = append(singleMatch, fmt.Sprintf(`[HTTP::path] eq "%s"`, *match.Path.Value))
			case gatewayapi.PathMatchRegularExpression:
				singleMatch = append(singleMatch, fmt.Sprintf(`[HTTP::path matches "%s"`, *match.Path.Value))
			}
		}
		if match.Headers != nil {
			for _, header := range match.Headers {
				matchType := gatewayapi.HeaderMatchExact
				if header.Type != nil {
					matchType = *header.Type
				}
				switch matchType {
				case gatewayapi.HeaderMatchExact:
					singleMatch = append(singleMatch, fmt.Sprintf(`[HTTP::header "%s"] eq "%s"`, header.Name, header.Value))
				case gatewayapi.HeaderMatchRegularExpression:
					singleMatch = append(singleMatch, fmt.Sprintf(`[HTTP::header "%s"] matches "%s"`, header.Name, header.Value))
				}
			}
		}
		if match.Method != nil {
			singleMatch = append(singleMatch, fmt.Sprintf(`[HTTP::method] eq "%s"`, *match.Method))
		}
		if match.QueryParams != nil {
			for _, queryParam := range match.QueryParams {
				matchType := gatewayapi.QueryParamMatchExact
				if queryParam.Type != nil {
					matchType = *queryParam.Type
				}
				switch matchType {
				case gatewayapi.QueryParamMatchExact:
					singleMatch = append(singleMatch, fmt.Sprintf(`[URI::query [HTTP::uri] "%s"] eq "%s"`, queryParam.Name, queryParam.Value))
				case gatewayapi.QueryParamMatchRegularExpression:
					singleMatch = append(singleMatch, fmt.Sprintf(`[URI::query [HTTP::uri] "%s"] matches "%s"`, queryParam.Name, queryParam.Value))
				}
			}
		}

		matchConditions = append(matchConditions, strings.Join(singleMatch, " and "))
	}
	return strings.Join(matchConditions, " or ")
}

func parseiRuleReqFilters(filters []gatewayapi.HTTPRouteFilter, hr gatewayapi.HTTPRoute) (string, error) {
	reqFilterActions := []string{}
	for _, filter := range filters {
		switch filter.Type {
		case gatewayapi.HTTPRouteFilterRequestHeaderModifier:
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

		case gatewayapi.HTTPRouteFilterRequestMirror:
			// filter.RequestMirror.BackendRef -> vs mirror?
			return "", fmt.Errorf("filter type '%s' not supported", gatewayapi.HTTPRouteFilterRequestMirror)
		case gatewayapi.HTTPRouteFilterRequestRedirect:
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
						return "", fmt.Errorf("invalid status %d for request redirect", *rr.StatusCode)
					}
				}
				action := fmt.Sprintf(`
					%s
					%s
					%s 
					%s
			        set url $rscheme://$rhostname:$rport$ruri
					HTTP::respond %d Location $url
					`, setScheme, setHostName, setUri, setPort, *rr.StatusCode)
				reqFilterActions = append(reqFilterActions, action)
			}
		case gatewayapi.HTTPRouteFilterExtensionRef:
			if er := filter.ExtensionRef; er != nil {
				pool := fmt.Sprintf("%s.%s", hr.Namespace, er.Name)
				reqFilterActions = append(reqFilterActions, fmt.Sprintf("pool /%s/%s; return", "cis-c-tenant", pool))
			}
		}
	}
	return strings.Join(reqFilterActions, "\n"), nil
}

func parseiRuleRespFilters(filters []gatewayapi.HTTPRouteFilter, hr gatewayapi.HTTPRoute) (string, error) {
	respFilterActions := []string{}
	for _, filter := range filters {
		switch filter.Type {
		case gatewayapi.HTTPRouteFilterResponseHeaderModifier:
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
	return strings.Join(respFilterActions, "\n"), nil
}

func parsePoolweight(backends []gatewayapi.HTTPBackendRef, hr *gatewayapi.HTTPRoute) string {
	poolWeights := []string{}
	for _, br := range backends {
		ns := hr.Namespace
		if br.Namespace != nil {
			ns = string(*br.Namespace)
		}
		svc := ActiveSIGs.GetService(utils.Keyname(ns, string(br.Name)))
		if svc != nil && !ActiveSIGs.CanRefer(hr, svc) {
			continue
		}
		// pn := strings.Join([]string{ns, string(br.Name)}, ".")
		pool := fmt.Sprintf("/%s/serviceMain/%s", ns, string(br.Name))
		weight := 1
		if br.Weight != nil {
			weight = int(*br.Weight)
		}
		poolWeights = append(poolWeights, fmt.Sprintf("%s %d", pool, weight))
	}

	return strings.Join(poolWeights, " ")
}

func parseiRulesFrom(className string, hr *gatewayapi.HTTPRoute, rlt map[string]interface{}) error {
	var tpl bytes.Buffer
	if err := iruleTemplate.ExecuteTemplate(&tpl, "irule.tmpl", hr); err != nil {
		return fmt.Errorf("cannot parse HttpRoute to iRule by template irule.tmpl")
	}

	name := hrName(hr)
	ruleObj := map[string]interface{}{
		"class": "iRule",
		"iRule": tpl.String(),
	}
	rlt["ltm/rule/"+name] = ruleObj
	return nil
}
