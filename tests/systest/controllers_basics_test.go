package main_test

import (
	"embed"
	"fmt"
	"time"

	f5_bigip "github.com/f5devcentral/f5-bigip-rest-go/bigip"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	//go:embed templates/basics/*.yaml
	yamlBasics embed.FS
	dataBasics map[string]interface{}
)

var _ = Describe("Controllers basic test", func() {

	dataBasics = map[string]interface{}{
		"namespace": map[string]interface{}{
			"name": "abcd",
		},
		"gatewayclass": map[string]interface{}{
			"name": "bigip",
		},
		"gateway": map[string]interface{}{
			"name": "mygateway",
			"listeners": []map[string]interface{}{
				{
					"name": "http",
					"port": 80,
				},
			},
			"ipAddresses": []string{"10.250.17.121"},
		},
		"referencegrant": map[string]interface{}{
			"name": "myreferencegrant",
		},
		"httproute": map[string]interface{}{
			"name":     "myhttproute",
			"hostname": "gateway.test.automation",
		},
		"service": map[string]interface{}{
			"name":     "test-service",
			"replicas": 3,
		},
	}

	BeforeEach(func() { setup(dataBasics) })
	AfterEach(func() { teardown(dataBasics) })

	It("resources deployed as expected", func() {
		checkVirtual(dataBasics)
		checkiRule(dataBasics)
		checkPool(dataBasics)
		checkVirtualAddress(dataBasics)
		slog.Infof("finished bigip resources checking")
	})
})

var _ = Describe("Controllers updating test", Ordered, func() {
	dataBasics = map[string]interface{}{
		"namespace": map[string]interface{}{
			"name": "abcd",
		},
		"gatewayclass": map[string]interface{}{
			"name": "bigip",
		},
		"gateway": map[string]interface{}{
			"name": "mygateway",
			"listeners": []map[string]interface{}{
				{
					"name": "http",
					"port": 80,
				},
			},
			"ipAddresses": []string{"10.250.17.121"},
		},
		"referencegrant": map[string]interface{}{
			"name": "myreferencegrant",
		},
		"httproute": map[string]interface{}{
			"name":     "myhttproute",
			"hostname": "gateway.test.automation",
		},
		"service": map[string]interface{}{
			"name":     "test-service",
			"replicas": 3,
		},
	}

	BeforeEach(func() { setup(dataBasics) })
	AfterEach(func() { teardown(dataBasics) })

	It("virtual address is replaced as expected", func() {
		dataBasics["gateway"] = map[string]interface{}{
			"name": "mygateway",
			"listeners": []map[string]interface{}{
				{
					"name": "http",
					"port": 80,
				},
			},
			"ipAddresses": []string{"10.250.17.122"},
		}

		for _, yaml := range []string{
			"templates/basics/gateway.yaml",
		} {
			f, err := yamlBasics.Open(yaml)
			Expect(err).To(Succeed())

			cs, err := k8s.LoadAndRender(ctx, f, dataBasics)
			Expect(err).To(Succeed())
			Expect(k8s.Apply(ctx, *cs)).To(Succeed())
		}

		checkVirtualAddress(dataBasics)
	})
})

var _ = Describe("Controller special cases", func() {
	When("there are multiple addrs and listeners in gateway", func() {
		dataBasics = map[string]interface{}{
			"namespace": map[string]interface{}{
				"name": "abcd",
			},
			"gatewayclass": map[string]interface{}{
				"name": "bigip",
			},
			"gateway": map[string]interface{}{
				"name": "mygateway",
				"listeners": []map[string]interface{}{
					{
						"name": "httpa",
						"port": 80,
					},
					{
						"name": "httpb",
						"port": 81,
					},
				},
				"ipAddresses": []string{"10.250.17.121", "10.250.17.122"},
			},
			"referencegrant": map[string]interface{}{
				"name": "myreferencegrant",
			},
			"httproute": map[string]interface{}{
				"name":     "myhttproute",
				"hostname": "gateway.test.automation",
			},
			"service": map[string]interface{}{
				"name":     "test-service",
				"replicas": 3,
			},
		}
		BeforeEach(func() { setup(dataBasics) })
		AfterEach(func() { teardown(dataBasics) })

		It("virtuals are created as expected", func() {
			checkVirtual(dataBasics)
			checkiRule(dataBasics)
			checkVirtualAddress(dataBasics)
		})
	})
})

var _ = Describe("Controllers random resource list", func() {
	dataBasics = map[string]interface{}{
		"namespace": map[string]interface{}{
			"name": "abcd",
		},
		"gatewayclass": map[string]interface{}{
			"name": "bigip",
		},
		"gateway": map[string]interface{}{
			"name": "mygateway",
			"listeners": []map[string]interface{}{
				{
					"name": "http",
					"port": 80,
				},
			},
			"ipAddresses": []string{"10.250.17.121"},
		},
		"referencegrant": map[string]interface{}{
			"name": "myreferencegrant",
		},
		"httproute": map[string]interface{}{
			"name":     "myhttproute",
			"hostname": "gateway.test.automation",
		},
		"service": map[string]interface{}{
			"name":     "test-service",
			"replicas": 3,
		},
	}
	BeforeEach(func() { setup(dataBasics) })
	AfterEach(func() { teardown(dataBasics) })

	for i := 0; i < 5; i++ {
		It("resources deployed as expected", func() {
			checkVirtual(dataBasics)
			checkiRule(dataBasics)
			checkVirtualAddress(dataBasics)
		})
	}
})

func setup(data map[string]interface{}) {
	for _, yaml := range []string{
		"templates/basics/gatewayclass.yaml",
		"templates/basics/gateway.yaml",
		"templates/basics/referencegrant.yaml",
		"templates/basics/httproute.yaml",
		"templates/basics/service.yaml",
	} {
		f, err := yamlBasics.Open(yaml)
		Expect(err).To(Succeed())
		cs, err := k8s.LoadAndRender(ctx, f, dataBasics)
		Expect(err).To(Succeed())
		Expect(k8s.Apply(ctx, *cs)).To(Succeed())
	}
}

func teardown(data map[string]interface{}) {
	for _, yaml := range []string{
		"templates/basics/service.yaml",
		"templates/basics/httproute.yaml",
		"templates/basics/referencegrant.yaml",
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
}

func checkVirtualAddress(data map[string]interface{}) {
	gwcVars := data["gatewayclass"].(map[string]interface{})
	gwVars := data["gateway"].(map[string]interface{})

	var kind, partition, subfolder string
	var body map[string]interface{}

	kind, partition, subfolder = "ltm/virtual-address", gwcVars["name"].(string), ""

	for _, name := range gwVars["ipAddresses"].([]string) {
		body = map[string]interface{}{}

		Eventually(bip.Check).
			ProbeEvery(time.Millisecond*500).
			WithContext(ctx).
			WithArguments(kind, name, partition, subfolder, body).
			WithTimeout(time.Second * 10).
			Should(Succeed())
	}
	slog.Infof("virtual-address is created as expected")
}

func checkVirtual(data map[string]interface{}) {
	gwVars := data["gateway"].(map[string]interface{})
	gwcVars := data["gatewayclass"].(map[string]interface{})
	hrVars := data["httproute"].(map[string]interface{})
	nsVars := data["namespace"].(map[string]interface{})

	var kind, partition, subfolder, name string
	var body map[string]interface{}

	kind, partition, subfolder = "ltm/virtual", gwcVars["name"].(string), ""
	for i, addr := range gwVars["ipAddresses"].([]string) {
		for _, listener := range gwVars["listeners"].([]map[string]interface{}) {
			name = fmt.Sprintf("gw.default.%s.%s.%d", gwVars["name"], listener["name"], i)
			body = map[string]interface{}{
				"partition":   gwcVars["name"],
				"destination": fmt.Sprintf("/%s/%s:80", gwcVars["name"], addr),
				"rules": []string{
					fmt.Sprintf("/%s/hr.%s.%s", gwcVars["name"], nsVars["name"], hrVars["name"]),
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
		}

	}

	slog.Infof("virtual is created as expected")
}

func checkiRule(data map[string]interface{}) {

	gwcVars := data["gatewayclass"].(map[string]interface{})
	hrVars := data["httproute"].(map[string]interface{})
	nsVars := data["namespace"].(map[string]interface{})

	var kind, partition, subfolder, name string
	var body map[string]interface{}

	kind, partition, subfolder = "ltm/rule", gwcVars["name"].(string), ""
	name = fmt.Sprintf("hr.%s.%s", nsVars["name"], hrVars["name"])
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
}

func checkPool(data map[string]interface{}) {
	svcVars := data["service"].(map[string]interface{})

	var kind, partition, subfolder, name string
	var body map[string]interface{}

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
}

// TODO: Add tests for
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
