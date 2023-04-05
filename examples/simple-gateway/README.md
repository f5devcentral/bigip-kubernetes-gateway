# Simple Gateway

In this example, we deploy a simple `Gateway` and relavent resources: `GatewayClass`, `HTTPRoute`, `Service` and `ReferenceGrant` for demonstrating a simple gateway functionality implemented via BIG-IP device.

## Running the Example

As the start, please follow the instruction for BIG-IP Kubernetes GatewayAPI Controller installation/deployment.

### 1. Deploy the `Service`

```shell
$ kubectl apply -f service.yaml
$ kubectl apply -f referencegrant.yaml
```

The *service.yaml* file defines an application named **test-service** implemented via NGINX and NJS. 

The application composes of a `Deployment` and a `Service`.
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

### 3. Deploy the `GatewayClass`

```shell
$ kubectl apply -f gatewayclass.yaml
```

The *gatewayclass.yaml* defines which partition would the gateway resources to be placed. 

The **controllerName** is immutable.

### 4. Deploy the `Gateway`

```shell
$ kubectl apply -f gateway.yaml
```

The *gateway.yaml* file defines the listeners and vip.

Multiple listeners and VIPs is supported.

**Note that, please update the `addresses` for your environment.**

### 5. Deploy the `HTTPRoute`

```shell
$ kubectl apply -f httproute.yaml
```

The *httproute.yaml* file defines how the traffic be routed to the backend service when a request reaches to the listener vip.

In this example, only the request path is '/path-test', would the request be routed to service *test-service*.

## Verify the Deployed Gateway

By checking the gateway works, run *curl* as:

```shell
$ curl http://10.250.17.143/path-test -H "Host: gateway.api"
{"queries":{},"headers":{"Host":"gateway.api","User-Agent":"curl/7.47.1","Accept":"*/*"},"version":"1.1","method":"GET","remote-address":"10.42.20.1","uri":"/path-test","server_name":"test-service-77478b5957-5xk5p"}
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
```
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
    }

    }

    when HTTP_RESPONSE {
    }
```

In the HTTP_REQUEST event, according to the .spec in `HTTPRoute`, we match the host, url and other conditions if needed for the traffic before forwarding it to the backend pool.