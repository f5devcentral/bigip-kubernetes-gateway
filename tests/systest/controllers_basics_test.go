package main_test

import (
	"fmt"
	"time"

	f5_bigip "github.com/f5devcentral/f5-bigip-rest-go/bigip"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controllers basic test", Ordered, func() {

	BeforeAll(func() {
		for _, yaml := range []string{
			"templates/basics/gatewayclass.yaml",
			"templates/basics/gateway.yaml",
			"templates/basics/httproute.yaml",
			"templates/basics/service.yaml",
		} {
			f, err := yamlBasics.Open(yaml)
			Expect(err).To(Succeed())
			cs, err := k8s.LoadAndRender(ctx, f, dataBasics)
			Expect(err).To(Succeed())
			Expect(k8s.Apply(ctx, *cs)).To(Succeed())
		}
	})

	AfterAll(func() {

		for _, yaml := range []string{
			"templates/basics/service.yaml",
			"templates/basics/httproute.yaml",
			"templates/basics/gateway.yaml",
			"templates/basics/gatewayclass.yaml",
		} {
			f, err := yamlBasics.Open(yaml)
			Expect(err).To(Succeed())

			cs, err := k8s.LoadAndRender(ctx, f, dataBasics)
			Expect(err).To(Succeed())
			Expect(k8s.Delete(ctx, *cs)).To(Succeed())
		}

		// make sure partition is removed finally
		gwcVars := dataBasics["gatewayclass"].(map[string]interface{})
		name := fmt.Sprintf(gwcVars["name"].(string))
		Eventually(bip.Exist).
			ProbeEvery(time.Millisecond*500).
			WithContext(ctx).
			WithArguments("sys/folder", name, "", "").
			WithTimeout(time.Second * 10).
			Should(Not(Succeed()))
	})

	It("resources deployed as expected", func() {
		checkResourcesAsExpected()
	})
})

func checkResourcesAsExpected() {
	gwVars := dataBasics["gateway"].(map[string]interface{})
	gwcVars := dataBasics["gatewayclass"].(map[string]interface{})
	hrVars := dataBasics["httproute"].(map[string]interface{})
	svcVars := dataBasics["service"].(map[string]interface{})

	var kind, partition, subfolder, name string
	var body map[string]interface{}

	kind, partition, subfolder = "ltm/virtual", gwcVars["name"].(string), ""
	name = fmt.Sprintf("gw.default.%s.http", gwVars["name"])
	body = map[string]interface{}{
		"partition":   gwcVars["name"],
		"destination": fmt.Sprintf("/%s/%s:80", gwcVars["name"], gwVars["ipAddress"]),
		"rules": []string{
			fmt.Sprintf("/%s/hr.default.%s", gwcVars["name"], hrVars["name"]),
		},
		"sourceAddressTranslation": map[string]interface{}{
			"type": "automap",
		},
	}

	Eventually(bip.Check).
		ProbeEvery(time.Millisecond*500).
		WithContext(ctx).
		WithArguments(kind, name, partition, subfolder, body).
		WithTimeout(time.Second * 10).
		Should(Succeed())

	slog.Infof("virtual is created as expected")

	kind, partition, subfolder = "ltm/rule", gwcVars["name"].(string), ""
	name = fmt.Sprintf("hr.default.%s", hrVars["name"])
	body = map[string]interface{}{
		"partition": gwcVars["name"],
	}

	Eventually(bip.Check).
		ProbeEvery(time.Millisecond*500).
		WithContext(ctx).
		WithArguments(kind, name, partition, subfolder, body).
		WithTimeout(time.Second * 10).
		Should(Succeed())

	rule, err := bip.Get(ctx, kind, name, partition, subfolder)
	Expect(err).To(Succeed())
	Expect(rule["apiAnonymous"]).Should(ContainSubstring(`[HTTP::host] matches "gateway.test.automation"`))
	Expect(rule["apiAnonymous"]).Should(ContainSubstring(`array set weights { /cis-c-tenant/default.test-service 1 }`))
	Expect(rule["apiAnonymous"]).Should(ContainSubstring(`[HTTP::path] starts_with "/path-test"`))
	Expect(rule["apiAnonymous"]).Should(ContainSubstring(`set pool $static::pools_0([expr {int(rand()*$static::pools_0_size)}])`))

	slog.Infof("irule is created as expected")

	kind, partition, subfolder = "ltm/pool", "cis-c-tenant", ""
	name = fmt.Sprintf("default.%s", svcVars["name"])
	body = map[string]interface{}{
		"partition": partition,
		"monitor":   "min 1 of { /Common/tcp }",
	}

	Eventually(bip.Check).
		ProbeEvery(time.Millisecond*500).
		WithContext(ctx).
		WithArguments(kind, name, partition, subfolder, body).
		WithTimeout(time.Second * 10).
		Should(Succeed())

	Eventually(func() bool {
		bc := f5_bigip.BIGIPContext{BIGIP: *bip.BIGIP, Context: ctx}
		members, err := bc.Members(name, partition, subfolder)
		Expect(err).To(Succeed())
		// slog.Infof("members: %d", len(members))
		return err == nil && len(members) == svcVars["replicas"].(int)
	}).WithContext(ctx).ProbeEvery(time.Millisecond * 500).WithTimeout(time.Second * 120).Should(BeTrue())

	slog.Infof("pool is created as expected")
	slog.Infof("finished bigip resources checking")
}

// TODO: Add tests for
//	updating gateway.yaml with addresses changed
//		-> check the virtual address is updated, legacy ones are removed.
// 	multiple addresses in the gateway
//		-> check multiple virtual created
// 	multiple gateways using the same address.
//		-> check virtual address are shared, and still exists when deleting one gateway.
//  multiple httproutes(of different classes) referring the same service
//		-> service is created and shared; still exists when deleting one httproute.
//  referencegrant for gateway <-> secret and httproute <-> service
//		-> check service is upserted or deleted as expected.
//  secret reconciler test
//		-> check gateway tls is up-to-date when secret is CUD-ed.
//  namespace reconciler test
//		-> check namespace label changes would trigger resources updating.
