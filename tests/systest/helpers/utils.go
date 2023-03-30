package helpers

import (
	"encoding/json"
	"reflect"

	"gopkg.in/yaml.v3"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func newRestConfig(kubeConfig string) (*rest.Config, error) {
	var config *rest.Config
	var err error
	if kubeConfig == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if nil != err {
			return nil, err
		}
	}
	return config, nil
}

func newDynamicClient(kubeConfig string) (*dynamic.DynamicClient, error) {
	config, err := newRestConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func newKubeClient(kubeConfig string) (*kubernetes.Clientset, error) {
	config, err := newRestConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
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

func deepequal(a, b interface{}) bool {
	ba, ea := json.Marshal(a)
	bb, eb := json.Marshal(b)
	if ea != nil || eb != nil {
		return false
	}
	if reflect.DeepEqual(ba, bb) {
		return true
	}

	// ja and jb have no type info,
	// so that []interface{}{"abc"} and []string{"abc"} are the same.
	var ja, jb interface{}
	ea, eb = json.Unmarshal(ba, ja), json.Unmarshal(bb, jb)
	if ea != nil || eb != nil {
		return false
	}
	return reflect.DeepEqual(ja, jb)
}
