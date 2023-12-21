package pkg

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_parseiRulesFrom(t *testing.T) {
	hryaml := `
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: myhttproute
  namespace: default
spec:
  parentRefs:
    - name: mygateway
      sectionName: http
  hostnames:
    - gateway.test.automation
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /test1
      backendRefs:
        - name: coffee
          port: 80
          weight: 1
        - name: tea
          port: 80
          weight: 9
    - matches:
        - path:
            type: PathPrefix
            value: /test2
      backendRefs:
        - name: coffee
          port: 80
          weight: 9
        - name: tea
          port: 80
          weight: 1
`

	var hr gatewayapi.HTTPRoute
	if err := load2runtimeObject([]byte(hryaml), &hr); err != nil {
		t.Logf("failed with msg: %s", err.Error())
		t.Fail()
	}

	rlt := map[string]interface{}{}
	if err := parseiRulesFrom("bigip", &hr, rlt); err != nil {
		t.Logf("failed with msg: %s", err.Error())
		t.Fail()
	}
	if raw, err := json.MarshalIndent(rlt, "", "  "); err != nil {
		t.Fail()
	} else {
		t.Logf("%s", raw)
		// TODO: do kinds of checks here.
	}
}

func yaml2json(data []byte) ([]byte, error) {
	var intf interface{}
	if err := yaml.Unmarshal(data, &intf); err != nil {
		return nil, err
	}

	if out, err := json.Marshal(intf); err != nil {
		return nil, err
	} else {
		return out, nil
	}
}

func load2runtimeObject(yamldata []byte, obj runtime.Object) error {
	if jdata, err := yaml2json(yamldata); err != nil {
		return err
	} else {
		_, _, err := unstructured.UnstructuredJSONScheme.Decode(jdata, nil, obj)
		if err != nil {
			return err
		}
	}
	return nil
}
