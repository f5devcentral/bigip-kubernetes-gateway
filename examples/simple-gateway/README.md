# Simple Gateway

In this example, we deploy a simple `Gateway` and relavent resources: `GatewayClass`, `HTTPRoute`, `Service` and `ReferenceGrant` for demonstrating a simple gateway functionality implemented via BIG-IP device.

## Running the Example

As the start, please follow the instruction for BIG-IP Kubernetes GatewayAPI Controller installation/deployment.

### 1. Deploy the `Service`

```shell
$ kubectl apply -f service.yaml
```

The *service.yaml* file defines 2 applications named **test-service** and **dev-service** implemented via NGINX and NJS. 

The applications composes of a `Deployment` and a `Service`.
The `ConfigMap` is used as `Deployment`'s *volumeMounts*, which defines nginx.conf and njs javascript logic.

When requesting to the service is requested, some request info would be responsed: 

```js
{
    'queries': r.args,
    'headers': r.headersIn,
    'version': r.httpVersion,
    'method': r.method,
    'remote-address': r.remoteAddress,
    'body': r.requestText,
    'uri': r.uri,
    'server_name': process.env['HOSTNAME']
}
```

In the service definition, `ClusterIP` service type is used, so, on BIG-IP, corresponding network resources would be setup for traffic connectivity.

### 2. Deploy the `ReferenceGrant`
The *referencegrant.yaml* file enables cross namespace resource reference, from *abcd* to *default*:

```yaml
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: abcd
  to:
    - group: ""
      kind: Service
```

The `ReferenceGrant` can be used to enable the references for:
* `Gateway` to `Secret` in *HTTPS* scenario, and
* `*Route` to Backends `Service`

More details about `ReferenceGrant`, see [here](https://gateway-api.sigs.k8s.io/api-types/referencegrant/).

### 3. Deploy the `GatewayClass`

```shell
$ kubectl apply -f gatewayclass.yaml
```

The *gatewayclass.yaml* defines which partition would the gateway resources to be placed. 

The **controllerName** is immutable.

### 4. Deploy the `Gateway`

*Note: Update the addresses in Gateway definition to your own*

```shell
$ kubectl apply -f gateway.yaml
```

The *gateway.yaml* file defines the listeners and vip.

Multiple listeners and VIPs is supported.

In the listener specification, we allow all `HTTPRoute`s attach this `Gateway`:

```
    allowedRoutes:
      namespaces:
        from: All
```

See [here](https://gateway-api.sigs.k8s.io/concepts/api-overview/#restricting-route-attachment) for more options of Route attachment.

`hostname: "*.api"` in the listener specification is optional, when it is defined, all the *hostanmes* defined in `HTTPRoute` specification will interact with it before forwarding the traffic to backends. That means if the *hostnames* in `HTTPRoute` mis-match the *hostname* in `Gateway`, the request would be dropped. See the details from [here](https://github.com/kubernetes-sigs/gateway-api/blob/main/apis/v1beta1/gateway_types.go#L182) and [here](https://gateway-api.sigs.k8s.io/api-types/httproute/#hostnames).

### 5. Deploy the `HTTPRoute`

```shell
$ kubectl apply -f httproute.yaml
```

The *httproute.yaml* file defines how the traffic be routed to the backend service when a request reaches to the listener vip.

In this example, only the request path starts with '/path-test', would the request be routed to service *test-service*, or else, defaultly, the traffic would be forwarded to *dev-service*:

```yaml
  rules:
    - matches:
      - path:
          type: PathPrefix
          value: /path-test
      backendRefs:
        - namespace: default
          name: test-service
          port: 80
    - backendRefs:
        - namespace: default
          name: dev-service
          port: 80
```

## Verify the Deployed Gateway

By checking the gateway works, run *curl* as:

```shell
# /path-test
$ curl http://10.250.17.143/path-test -H "Host: gateway.api"
{"queries":{},"headers":{"Host":"gateway.api","User-Agent":"curl/7.47.1","Accept":"*/*"},"version":"1.1","method":"GET","remote-address":"10.42.20.1","uri":"/path-test","server_name":"test-service-77478b5957-5xk5p"}

# /other-path
$ curl http://10.250.17.143/other-path -H "Host: gateway.api"
{"queries":{},"headers":{"Host":"gateway.api","User-Agent":"curl/7.47.1","Accept":"*/*"},"version":"1.1","method":"GET","remote-address":"10.42.20.1","uri":"/other-path","server_name":"dev-service-77b97c94dc-wfbjc"}
```

From the response json-format body, we can see the request information, like *server_name*, *Host*, *uri*, etc.

On BIG-IP, the gateway functionality is delivered with *Virtual* *iRule* and *Pool*:

*The following resources can be retrieved via iControl Rest:*

*`curl -k -u admin:xxx https://<BIG-IP>/mgmt/tm/ltm/<resource_type>`*
```json
{
    "kind": "tm:ltm:virtual:virtualstate",
    "name": "gw.default.mygateway.listenerx",
    "partition": "bigip",
    "fullPath": "/bigip/gw.default.mygateway.listenerx",
    ....
    "addressStatus": "yes",
    "ipProtocol": "tcp",
    "rateLimit": "disabled",
    "rateLimitDstMask": 0,
    "rateLimitMode": "object",
    "rateLimitSrcMask": 0,
    "sourceAddressTranslation": {
        "type": "automap"
    },
    "profilesReference": {
        "link": "..",
        "isSubcollection": true
    }
}

{
    "kind": "tm:ltm:pool:poolstate",
    "name": "default.test-service",
    "partition": "cis-c-tenant",
    "fullPath": "/cis-c-tenant/default.test-service",
    ...
    "monitor": "min 1 of { /Common/tcp }",
    "queueDepthLimit": 0,
    "queueOnConnectionLimit": "disabled",
    "queueTimeLimit": 0,
    "reselectTries": 0,
    "serviceDownAction": "none",
    "slowRampTime": 10,
    "membersReference": {
        "link": "...",
        "isSubcollection": true
    }
}

{
    "kind": "tm:ltm:rule:rulestate",
    "name": "hr.abcd.myroute",
    "partition": "bigip",
    "fullPath": "/bigip/hr.abcd.myroute",
    "generation": 115,
    "selfLink": "...",
    "apiAnonymous": "..."
}

```

The content of "apiAnonymous" is:

```c
when RULE_INIT {
    array unset weights *
    array unset static::pools_0 *
    set index 0

    array set weights { /cis-c-tenant/default.test-service 1 }
    foreach name [array names weights] {
        for { set i 0 }  { $i < $weights($name) }  { incr i } {
            set static::pools_0($index) $name
            incr index
        }
    }
    set static::pools_0_size [array size static::pools_0]
}

when HTTP_REQUEST {

    if { [HTTP::host] matches "gateway.api" }{
        if { [HTTP::path] starts_with "/path-test" } {

            if { $static::pools_0_size != 0 }{
                set pool $static::pools_0([expr {int(rand()*$static::pools_0_size)}])
                pool $pool
            }
            return
        }

        
        if { [HTTP::path] starts_with "/" } {
        
            if { $static::pools_1_size != 0 }{
                set pool $static::pools_1([expr {int(rand()*$static::pools_1_size)}])
                pool $pool
            }
            return
        }
    }
}

when HTTP_RESPONSE {
}
```

In the HTTP_REQUEST event, according to the .spec in `HTTPRoute`, we match the host, url and other conditions if needed for the traffic before forwarding it to the backend pool.