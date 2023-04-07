# Traffic Shaping

`HTTPRoute` as well as other *Route define kinds of *Rule* for forwarding the traffic to backends.

In the *Rule*, we can define not only *Match*, but also *Filter*.

In this example, we enumerate kinds of `HTTPRoute`'s matches and filters for readers to refer.

## Run the Example

### 1. Create the `GatewayClass` and `Gateway`

```shell
$ kubectl apply -f gatewayclass.yaml
$ kubectl apply -f gateway.yaml
```

### 2. Create the `Service`

```shell
$ kubectl apply -f service.yaml
```

### 3. Apply Kinds of `HTTPRoute`

```shell
$kubectl apply -f <hrs-xx>.yaml
```

There are 2 set of `HTTPRoute`s: *-matches-* and *-filters-*, 

*-matches-* means request matching before forwarding; *-filters-* means request modifying before forwarding.

The *hrs-.yaml* targets are summarized as following table:
| File | Matches | Filters | Action |
| :--- | :--- |:--- |:---: |
|hrs-matches-header.yaml|-H "test: automation"|--|forwarding to test-service|
|hrs-matches-mix.yaml|-X GET or -X OPTIONS|--|forwarding to test-service|
||/?test=automation or /path-test* |--|forwarding to test-service|
|| by default |--|forwarding to test-service|
|hrs-matches-query.yaml|/?test=automation|--|forwarding to test-service|
|hrs-matches-path.yaml |/path-test*|--|forwarding to test-service|
|hrs-matches-method.yaml|-X GET or -X OPTIONS|--|forwarding to test-service|
|hrs-filters-header.yaml|--|test=automation; dev=agile|forwarding to test-service|
|hrs-filters-extensionref.yaml|--|forwarding to test-service||
|hrs-filters-request-redirect.yaml|--|redirect to https://www.example.com||

See the full specification of `HTTPRoute`'s *Match* and *Filter* from [here](https://github.com/kubernetes-sigs/gateway-api/blob/c09effdda6c58945fe2896d372cdd5490fdd3a9d/apis/v1beta1/httproute_types.go#L123)