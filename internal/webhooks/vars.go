package webhooks

import "sigs.k8s.io/controller-runtime/pkg/manager"

var (
	validateMap = map[string]bool{
		VK_gateway_gatewayClassName:              false,
		VK_gateway_listeners_tls_certificateRefs: false,
		VK_httproute_parentRefs:                  false,
		VK_httproute_rules_backendRefs:           false,
	}
)

const (
	VK_gateway_gatewayClassName              = "gateway.gatewayClassName"
	VK_gateway_listeners_tls_certificateRefs = "gateway.listeners.tls.certificateRefs"
	VK_httproute_parentRefs                  = "httproute.parentRefs"
	VK_httproute_rules_backendRefs           = "httproute.rules.backendRefs"
)

var (
	WebhookManager manager.Manager
)
