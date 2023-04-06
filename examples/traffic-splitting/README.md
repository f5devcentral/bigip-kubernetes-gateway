# Traffic Splitting

In this example, we show a useful practice of GatewayAPI.

Basically, the main concepts of the resources are same as that in example [Simple Gateway](../simple-gateway). Go through it if needed.

To be focused, the resources `Gateway` `HTTPRoute` and `Service` are all deployed in the same namespace, thus, `ReferenceGrant` resource is no longer needed in this example for cross namespace reference.

## Run the Example

### 1. Deploy the Services

```shell
$ kubectl apply -f services.yaml
```

*services.yaml* defines 2 applications **tea** and **coffee**.

Both of the applications are based on NGINX + NJS. See [here](../simple-gateway/README.md#1-deploy-the-service) for more details about the implementation.

Service *tea* uses `NodePort` service type, while service *coffee* uses `ClusterIP`. Both of the service types are supported.

### 2. Deploy the GatewayAPI Resources

*Note: Update the addresses in Gateway definition to your own*

```shell
$ kubectl apply -f gatewayapis.yaml
```

For simplicity, the resources `GatewayClass`, `Gateway` and `HTTPRoute` are placed in a single yaml file. In the production case, those resources are actually maintained by different roles/teams.

By running the above command, the gateway functionality is setup together in one process on BIG-IP, although the controller is actually receiving 3 resource events.

Note that, the controller is not sensitive to the order of resource events, unless the startup parameter **--validates**, see the [Parameter](https://gateway-api.f5se.io/deploy/parameters/) for more details.

In `HTTPRoute` definition, multiple backends are defined, with different *weight*. The *weight* is used to calculate the percentage of traffic splitting:

```yaml
      backendRefs:
        - name: coffee
          port: 80
          weight: 1
        - name: tea
          port: 80
          weight: 9
```

## Verify the Deployed Gateway

Use *curl* to verify the deployed gateway:

```shell
# when the request uri is /test1, 90% of requests are answered by TEA.
$ curl http://10.250.17.143/test1 -H "Host: gateway.api"
{"queries":{},"headers":{"Host":"gateway.api","User-Agent":"curl/7.47.1","Accept":"*/*"},"version":"1.1","method":"GET","remote-address":"10.42.1.1","uri":"/test1","server_name":"TEA"}
...
$ curl http://10.250.17.143/test1 -H "Host: gateway.api"
{"queries":{},"headers":{"Host":"gateway.api","User-Agent":"curl/7.47.1","Accept":"*/*"},"version":"1.1","method":"GET","remote-address":"10.42.20.1","uri":"/test1","server_name":"COFFEE"}
```

```shell
# when the request uri is /test2, 90% of requests are answered by COFFEE.
$ curl http://10.250.17.143/test2 -H "Host: gateway.api"
{"queries":{},"headers":{"Host":"gateway.api","User-Agent":"curl/7.47.1","Accept":"*/*"},"version":"1.1","method":"GET","remote-address":"10.42.20.1","uri":"/test2","server_name":"COFFEE"}
...
$ curl http://10.250.17.143/test2 -H "Host: gateway.api"
{"queries":{},"headers":{"Host":"gateway.api","User-Agent":"curl/7.47.1","Accept":"*/*"},"version":"1.1","method":"GET","remote-address":"10.42.0.0","uri":"/test2","server_name":"TEA"}
```

Equally, we can use [iControl Rest way in Simple Gateway example](../simple-gateway/README.md#verify-the-deployed-gateway) to verify the resources created on BIG-IP.

Specially, the iRule created on BIG-IP is: 

```c
when RULE_INIT {

    array unset weights *
    array unset static::pools_0 *
    set index 0

    array set weights { /cis-c-tenant/default.coffee 1 /cis-c-tenant/default.tea 9 }
    foreach name [array names weights] {
        for { set i 0 }  { $i < $weights($name) }  { incr i } {
            set static::pools_0($index) $name
            incr index
        }
    }
    set static::pools_0_size [array size static::pools_0]

    array unset weights *
    array unset static::pools_1 *
    set index 0

    array set weights { /cis-c-tenant/default.coffee 9 /cis-c-tenant/default.tea 1 }
    foreach name [array names weights] {
        for { set i 0 }  { $i < $weights($name) }  { incr i } {
            set static::pools_1($index) $name
            incr index
        }
    }
    set static::pools_1_size [array size static::pools_1]
}

when HTTP_REQUEST {
    if { [HTTP::host] matches "gateway.api" }{
        if { [HTTP::path] starts_with "/test1" } {
        
            if { $static::pools_0_size != 0 }{
                set pool $static::pools_0([expr {int(rand()*$static::pools_0_size)}])
                pool $pool
            }
            return
        }
        
        if { [HTTP::path] starts_with "/test2" } {
        
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

According to specifications in `HTTPRoute`, we calculate the percentage of traffic, and forward traffic to different backends. 
