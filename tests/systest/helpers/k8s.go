package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"text/template"

	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8SHelper struct {
	kubeconfig   string
	clientset    *dynamic.DynamicClient
	apiResources []*metav1.APIResourceList
}

type Configs []*unstructured.Unstructured

func NewK8SHelper(ctx context.Context, kubeconfig string) (*K8SHelper, error) {
	slog := utils.LogFromContext(ctx)
	dc, err := newDynamicClient(kubeconfig)
	if err != nil {
		return nil, err
	}
	kc := &K8SHelper{
		kubeconfig: kubeconfig,
		clientset:  dc,
	}
	// if kc.clientset == nil.. newDynamicClient panic if fails
	client, err := newKubeClient(kubeconfig)
	if err != nil {
		return nil, err
	}
	_, rs, err := client.DiscoveryClient.ServerGroupsAndResources()
	if err != nil {
		slog.Errorf("failed to list api resources: %s", err)
		return nil, err
	} else {
		kc.apiResources = rs
	}
	return kc, nil
}

func (k *K8SHelper) Apply(ctx context.Context, configs Configs) error {
	slog := utils.LogFromContext(ctx)

	for _, confyaml := range configs {
		apiver := confyaml.GetAPIVersion()
		ns := confyaml.GetNamespace()
		keyname := utils.Keyname(ns, confyaml.GetName())
		gvk := schema.FromAPIVersionAndKind(apiver, confyaml.GetKind())
		gvr := k.gvk2gvr(gvk)
		// kubectl apply xx resources in prior is not allowed for:
		// {
		// 	Type: "FieldManagerConflict",
		// 	Message: "conflict with \"kubectl-client-side-apply\" using gateway.networking.k8s.io/v1beta1",
		// 	Field: ".spec.addresses",
		// }
		applyOps := metav1.ApplyOptions{FieldManager: utils.Keyname(gvr.Group, gvr.Version)}

		_, err := k.clientset.Resource(gvr).Namespace(ns).Apply(ctx, confyaml.GetName(), confyaml, applyOps)
		if err == nil {
			slog.Infof("applied %s/%s %s", gvk.GroupVersion().String(), gvk.Kind, keyname)
		} else {
			slog.Errorf("failed to apply %s/%s %s: %s", gvk.GroupVersion().String(), gvk.Kind, keyname, err.Error())
			return err
		}
	}

	return nil
}

func (k *K8SHelper) Delete(ctx context.Context, configs Configs) error {
	slog := utils.LogFromContext(ctx)
	for i := len(configs) - 1; i >= 0; i-- {
		confyaml := configs[i]
		apiver := confyaml.GetAPIVersion()
		ns := confyaml.GetNamespace()
		keyname := utils.Keyname(ns, confyaml.GetName())
		gvk := schema.FromAPIVersionAndKind(apiver, confyaml.GetKind())
		gvr := k.gvk2gvr(gvk)

		err := k.clientset.Resource(gvr).Namespace(ns).Delete(ctx, confyaml.GetName(), metav1.DeleteOptions{})
		if err == nil {
			slog.Infof("deleted %s/%s %s", gvk.GroupVersion().String(), gvk.Kind, keyname)
		} else {
			slog.Errorf("failed to delete %s/%s %s: %s", gvk.GroupVersion().String(), gvk.Kind, keyname, err.Error())
			return err
		}
	}
	return nil
}

func (k *K8SHelper) Loads(ctx context.Context, yaml string) (*Configs, error) {
	configs := []*unstructured.Unstructured{}
	slog := utils.LogFromContext(ctx)

	resources := strings.Split(yaml, "---")
	for _, res := range resources {
		trimed := res
		trimed = strings.Trim(trimed, " ")
		trimed = strings.Trim(trimed, "\n")
		trimed = strings.Trim(trimed, "\t")
		if trimed == "" {
			continue
		}
		jd, err := yaml2json([]byte(res))
		if err != nil {
			slog.Errorf("failed to convert yaml content to json: %s: %s", res, err.Error())
			return nil, err
		}
		var j map[string]interface{}
		if err := json.Unmarshal(jd, &j); err != nil {
			slog.Errorf("failed to unmarshal %s: %s", jd, err.Error())
			return nil, err
		}
		confyaml := unstructured.Unstructured{
			Object: j,
		}
		if confyaml.GetNamespace() == "" && k.namespaceScoped(confyaml.GroupVersionKind()) {
			confyaml.SetNamespace("default")
		}
		configs = append(configs, &confyaml)
	}

	if len(configs) == 0 {
		slog.Warnf("found %d resources", len(configs))
	}
	return (*Configs)(&configs), nil
}

func (k *K8SHelper) Load(ctx context.Context, fs io.Reader) (*Configs, error) {
	slog := utils.LogFromContext(ctx)

	if b, err := io.ReadAll(fs); err != nil {
		slog.Errorf("failed to read: %s", err.Error())
		return nil, err
	} else {
		return k.Loads(ctx, string(b))
	}
}

func (k *K8SHelper) LoadAndRender(ctx context.Context, fs io.Reader, data map[string]interface{}) (*Configs, error) {
	slog := utils.LogFromContext(ctx)

	if b, err := io.ReadAll(fs); err != nil {
		slog.Errorf("failed to read: %s", err.Error())
		return nil, err
	} else if s, err := k.render(ctx, string(b), data); err != nil {
		slog.Errorf("failed to render: %s", err.Error())
		return nil, err
	} else {
		// slog.Infof(s)
		return k.Loads(ctx, s)
	}
}

func (k *K8SHelper) render(ctx context.Context, j2yaml string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("").Parse(j2yaml)
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, data); err != nil {
		return "", err
	}
	return buff.String(), nil
}

func (k *K8SHelper) gvk2gvr(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	for _, rs := range k.apiResources {
		if gvk.GroupVersion().String() != rs.GroupVersion {
			continue
		}
		for _, r := range rs.APIResources {
			if r.Kind == gvk.Kind {
				return schema.GroupVersionResource{
					Group:    gvk.Group,
					Version:  gvk.Version,
					Resource: r.Name,
				}
			}
		}
		break
	}
	// should not happen
	p, _ := meta.UnsafeGuessKindToResource(gvk)
	return p
}

func (k *K8SHelper) namespaceScoped(gvk schema.GroupVersionKind) bool {
	for _, rs := range k.apiResources {
		if gvk.GroupVersion().String() != rs.GroupVersion {
			continue
		}
		for _, r := range rs.APIResources {
			if r.Kind == gvk.Kind {
				return r.Namespaced
			}
		}
		break
	}

	// should not happen
	return false
}
