# System Test for bigip-kubernetes-gateway

We leverage [Ginkgo](https://onsi.github.io/ginkgo/) to implement the system test framework. 

## Usage

* Run `ginkgo build` to build the `systest.test` binary which can run anywhere independently with running environment.

* Configure `test-config.yaml` file which is read by `systest.test`.

   ```yaml
   # kube configuration file path
   kubeConfig: /Users/zong/.kube/config
   # bigip configuration, username defaults to 'admin'
   bigipConfig:
    ipAddress: 10.250.2.219
    port: 443
    password: P@ssw0rd123
   ```

  Notes:
   * the file name must be `test-config.yaml`
   * the file must be placed under the same directory of `systest.test`

## Run the tests

```shell
$ ./systest.test --ginkgo.v
Running Suite: BigipKubernetesGateway Suite - /Users/zong/Downloads/tmp
=======================================================================
Random Seed: 1679619956

Will run 10 of 10 specs
------------------------------
[BeforeSuite]
/Users/zong/GitRepos/zongzw/bigip-kubernetes-gateway/tests/systest/bigip_kubernetes_gateway_suite_test.go:25
2023/03/24 09:05:56.125083  [INFO] [-] loaded test configuration: {/Users/zong/.kube/config {admin P@ssw0rd123 10.250.2.219 443}}
2023/03/24 09:05:57.133906  [INFO] [-] initialized k8s and bigip helpers
[BeforeSuite] PASSED [1.009 seconds]
------------------------------
Webhhooks Validating GatewayClass when be referred by gateways gatewayclass cannot be deleted
/Users/zong/GitRepos/zongzw/bigip-kubernetes-gateway/tests/systest/webhooks_gatewayclass_test.go:52
2023/03/24 09:05:57.174686  [INFO] [-] applied gateway.networking.k8s.io/v1beta1/GatewayClass bigip
2023/03/24 09:05:57.221584  [INFO] [-] applied gateway.networking.k8s.io/v1beta1/Gateway default/mygateway
...
2023/03/24 09:06:58.073917  [INFO] [-] deleted gateway.networking.k8s.io/v1beta1/HTTPRoute default/myhttproute
2023/03/24 09:06:58.109289  [INFO] [-] deleted gateway.networking.k8s.io/v1beta1/Gateway default/mygateway
2023/03/24 09:06:58.142941  [INFO] [-] deleted gateway.networking.k8s.io/v1beta1/GatewayClass bigip
• [7.525 seconds]
------------------------------
[AfterSuite]
/Users/zong/GitRepos/zongzw/bigip-kubernetes-gateway/tests/systest/bigip_kubernetes_gateway_suite_test.go:47
[AfterSuite] PASSED [0.000 seconds]
------------------------------

Ran 10 of 10 Specs in 65.068 seconds
SUCCESS! -- 10 Passed | 0 Failed | 0 Pending | 0 Skipped
PASS
```

