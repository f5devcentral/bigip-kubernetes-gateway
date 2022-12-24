# Gateway API Compatibility(v0.5.1)

This document describes which Gateway API resources BIG-IP Kubernetes Gateway supports and the extent of that support.

## Summary

| Resource | Support Status |
|-|-|
| [GatewayClass](#gatewayclass) | Partially supported |
| [Gateway](#gateway) | Partially supported |
| [HTTPRoute](#httproute) | Partially supported |
| [TLSRoute](#tlsroute) | Not supported, experimental in v0.5.1 |
| [TCPRoute](#tcproute) | Not supported, experimental in v0.5.1 |
| [UDPRoute](#udproute) | Not supported, experimental in v0.5.1 |

## Terminology

We use the following words to describe support status:
- *Supported*. The resource or field is fully supported and conformant to the Gateway API specification.
- *Partially supported*. The resource or field is supported partially or with limitations. It will become fully supported in future releases.
- *Not supported*. The resource or field is not yet supported. It will become partially or fully supported in future releases.

Note: it might be possible that BIG-IP Kubernetes Gateway will never support some resources and/or fields of the Gateway API. We will document these decisions on a case by case basis.

## Resources

Below we list the resources and the support status of their corresponding fields. 

For a description of each field, visit the [Gateway API documentation](https://gateway-api.sigs.k8s.io/references/spec/). 

### GatewayClass 

> Status: Partially supported.

BIG-IP Kubernetes Gateway supports the coexistence of multiple gatewayClasses, and their `controllerName` field determines which controller handles this gatewayclass resource. Each GatewayClass is represented as an independent partition on BIG-IP.

Fields:
* `spec`
	* `controllerName` - supported.
	* `parametersRef` - will not support. 
	* `description` - not supported.
* `status` - not supported.

### Gateway

> Status: Partially supported.

BIG-IP Kubernetes Gateway supports most Gateway Spec definitions. The Gateway resource will be parsed as a virtual resource on the BIG-IP device as an application entry for external connections.

Fields:
* `spec`
	* `gatewayClassName` - supported.
	* `listeners`
		* `name` - supported.
		* `hostname` - not supported.
		* `port` - supported.
		* `protocol` - partially supported. Allowed values: `HTTP`.
		* `tls` - not supported.
		  * `options` - not supported.
		* `allowedRoutes` - not supported. 
	* `addresses` - partially upported.
	    * type `IPAddress`: supported.
		* type `Hostname`: will not support.
		* type `NamedAddress`: will not support.
* `status`
  * `addresses` - not supported.
  * `conditions` - not supported.
  * `listeners`
	* `name` - not supported.
	* `supportedKinds` - not supported.
	* `attachedRoutes` - not supported.
	* `conditions` - not supported.

### HTTPRoute

> Status: Partially supported.

Fields:
* `spec`
  * `parentRefs` - partially supported.
    * `group` `kind`: partially supported, only for `Gateway`.
	* `namespace` `name`: supported.
    * `sectionName` must always be set.
	* `port`: will not support. 
  * `hostnames` - supported. 
  * `rules`
	* `matches`
	  * `path` - supported.
	  * `headers` - supported.
	  * `queryParams` - supported. 
	  * `method` -  supported.
	* `filters`
		* `type` - supported.
		* `requestRedirect` - supported. 
		* `requestHeaderModifier` - supported.
        * `requestMirror` - not supported.
        * `urlRewrite` - supported, experimental in v0.5.1.
        * `extensionRef` - partially supported, only v1.Service.
	* `backendRefs` - partially supported.
	    * `group` `kind` partially supported. only v1.Service. 
		* Backend ref `filters` will not support.
* `status` - not supported.
  * `parents` - not supported.
	* `parentRef` - not supported.
	* `controllerName` - not supported.
	* `conditions` - not supported.

### TLSRoute

> Status: Not supported.

### TCPRoute

> Status: Not supported.

### UDPRoute

> Status: Not supported.
