package main_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Webhhooks Validating GatewayClass", Ordered, func() {

	BeforeAll(func() {
		for _, yaml := range []string{
			"templates/basics/gatewayclass.yaml",
			"templates/basics/gateway.yaml",
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
	Context("when be referred by gateways", func() {
		It("gatewayclass cannot be deleted", func() {

			yaml := "templates/basics/gatewayclass.yaml"
			f, err := yamlBasics.Open(yaml)
			Expect(err).To(Succeed())

			cs, err := k8s.LoadAndRender(ctx, f, dataBasics)
			Expect(err).To(Succeed())
			Eventually(k8s.Delete).WithContext(ctx).WithArguments(*cs).WithTimeout(time.Second * 10).Should(Not(Succeed()))
		})
	})
})
