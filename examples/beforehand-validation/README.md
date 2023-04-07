# Before Hand Validation

In some cases, users may want to perform checks on their defined YAML files before making actual changes to Kubernetes. 

If the YAML content does not meet the relevant conditions, the controller can provide feedback so that the YAML content can be adjusted accordingly.

In controller, we use `Webhook` mechanism to achieve this. To use this feature, you need to start the controller with parameter **--validates**. see the [Parameter](https://gateway-api.f5se.io/deploy/parameters/) for more details.

```shell
  -validates string
    	The items to validate synchronizingly, on operations concating multiple values with ',', valid values: httproute.parentRefs,httproute.rules.backendRefs,gateway.gatewayClassName,gateway.listeners.tls.certificateRefs
```

When '--validates' is used at controller starts, the controller will check the referred resources' situation. There are different checkings for different references. Let's split the --validates values by ',', and explain each of them:

* *httproute.parentRefs*

  When this option is appended, the reference between `Gateway` and `HTTPRoute` would be checked.

  * If the `Gateway` doesn't exist, `HTTPRoute` upsert(update & insert) request would be denied.
  * If there are still `HTTPRoute`s referring to `Gateway`, the `Gateway`'s deletion request would be denied.
  * 

* *httproute.rules.backendRefs*

  When this option is appended, the reference between `HTTPRoute` and `Service` would be checked.

  If there are no `Service` existing, `HTTPRoute` upsert request would be denied.

  If there 

* *gateway.gatewayClassName*

  When this option is appended,
  
  * The *gatewayClassName* is not allowed to change.
  * If there are still `Gateway`s referring to `GatewayClass`, the `GatewayClass` is not permitted to delete.
  * If there is no `GatewayClass`, the referred `Gateway` upsert request would be denied.

* *gateway.listeners.tls.certificateRefs*:

  When this option is appended, the reference between `Gateway` and `Secret` would be checked in HTTPS scenarios.

  * If the `Secret` doesn't exist or is invalid, the `Gateway` upsert would be denied.

Note that:

* The reference failures because of `ReferenceGrant` or `allowedRoutes` would be considered as invalid, the requests would be denied.
* The before hand validation can be enabled or disabled by --validates parameters.
* All the possible invalidation will be reported in a single check, for example, if `Gateway` refers to 3 `Secret`, 2 of them are invalid, the 2 `Secret` reference failure will be reported together, users need not to run the request 2 times to get all exceptions.
* Checks on *Group* and *Kind* of references will be done for the validation. For example, the `HTTPRoute`'s *parentRefs* can only be `Gateway`, although the Gateway API specification support other possibilities.
* Currently, we only do validations for Gateway API resources as `GatewayClass`, `Gateway`, `HTTPRoute` and `ReferenceGrant`. The `Service` and `Secret` are out of the scope.

After telling the long description of this feature, let's run the example for demonstration.

## Run the Example

### Start the Controller with --validates

See [here](https://github.com/f5devcentral/bigip-kubernetes-gateway/blob/master/deploy/3.deploy-bigip-kubernetes-gateway-controller.yaml#L163) for updating the parameters.

### Create `Gateway` without `GatewayClass`

Keep the `GatewayClass` not exist, and run the command:

```shell
$ kubectl apply -f gateway.yaml
Error from server (gatewayclass 'bigip' not found): error when creating "gateway.yaml": admission webhook "vgw.kb.io" denied the request: gatewayclass 'bigip' not found
```

### Create `HTTPRoute` without `Gateway`

```shell
$ kubectl apply -f httproute.yaml
namespace/abcd created
Error from server (no gateway 'default/mygateway' found): error when creating "httproute.yaml": admission webhook "vhr.kb.io" denied the request: no gateway 'default/mygateway' found
```

### Delete `Gateway` with `HTTPRoute` still referring to

```shell
$ kubectl delete -f gateway.yaml
Error from server (still referred by abcd/myroute): error when deleting "gateway.yaml": admission webhook "vgw.kb.io" denied the request: still referred by abcd/myroute
```


### Delete `GatewayClass` with `Gateway` still referring to

```shell
$ kubectl delete -f gatewayclass.yaml
Error from server (still be referred by [default/mygateway]): error when deleting "gatewayclass.yaml": admission webhook "vgwc.kb.io" denied the request: still be referred by [default/mygateway]
```

See the above feedbacks from CLI, you may notice the C/U/D requests are denied, and the failure reasons are printed immediately before we do actual changes to the very Kubernetes cluster.

To fix it, request them with the conditions satisfied, for example, the last one, you may delete the `Gateway`s mentioned in the output *default/mygateway*, and do the deletion of `GatewayClass` again.