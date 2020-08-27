package converter

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/oam-kubernetes-runtime/apis/core/v1alpha2"
)

// v1alpha1Component is field descriptions of old v1alpha1 ApplicationConfigurations
type v1alpha1Component interface{}

// v1alpha2Component is field descriptions of new v1alpha2 ApplicationConfigurations
type v1alpha2Component map[string]interface{}

// v1alpha1Trait is field descriptions of old v1alpha1 ApplicationConfigurations
type v1alpha1Trait interface{}

// v1alpha2Trait is field descriptions of new v1alpha2 ApplicationConfigurations
type v1alpha2Trait map[string]interface{}

// Converter is the plugin that convert v1alpha1 OAM types to v1alpha2 types
type Converter interface {
	// ConvertComponent converts spec.components from v1alpha1 types to v1alpha2 types
	// in ApplicationConfigurations and return a v1alpha2 component CR.
	ConvertComponent(v1alpha1Component) (v1alpha2Component, v1alpha2.Component, error)

	// ConvertTrait converts spec.components[*].traits from v1alpha1 types to v1alpha2 types
	// in ApplicationConfigurations.
	ConvertTrait(v1alpha1Trait) (v1alpha2Trait, error)
}

// ExamplePlugin is conversion method receiver
type ExamplePlugin struct {
}

// ConvertComponent converts spec.components from v1alpha1 types to v1alpha2 types
// in ApplicationConfigurations and return a v1alpha2 component CR.
func (p *ExamplePlugin) ConvertComponent(comp v1alpha1Component) (v1alpha2Component, v1alpha2.Component, error) {
	c, _ := comp.(map[string]interface{})

	var v1alpha2Comp v1alpha2.Component

	name, _, _ := unstructured.NestedString(c, "componentName")
	v1alpha2Comp.Name = name
	v1alpha2Comp.Namespace = "default"

	// Here just for demo.
	// You should get the existing deployment in the cluster directly, and assign it to v1alpha2 component.
	instanceName, _, _ := unstructured.NestedString(c, "instanceName")
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceName,
			Namespace: "default",
		},
	}
	v1alpha2Comp.Spec.Workload = runtime.RawExtension{Object: deploy}

	unstructured.RemoveNestedField(c, "parameterValues")
	unstructured.RemoveNestedField(c, "instanceName")

	return c, v1alpha2Comp, nil
}

// ConvertTrait converts spec.components[*].traits from v1alpha1 types to v1alpha2 types
// in ApplicationConfigurations.
func (p *ExamplePlugin) ConvertTrait(tr v1alpha1Trait) (v1alpha2Trait, error) {
	t, _ := tr.(map[string]interface{})
	v1alpha2Trait := make(map[string]interface{}, 0)

	_ = unstructured.SetNestedField(v1alpha2Trait, "core.oam.dev/v1alpha2", "apiVersion")
	_ = unstructured.SetNestedField(v1alpha2Trait, "RolloutTrait", "kind")

	v1alpha2Metadata := make(map[string]interface{}, 0)
	name, _, err := unstructured.NestedString(t, "name")
	if err != nil {
		return nil, err
	}
	_ = unstructured.SetNestedField(v1alpha2Metadata, name, "name")
	_ = unstructured.SetNestedField(v1alpha2Trait, v1alpha2Metadata, "metadata")

	v1alpha2Spec := make(map[string]interface{}, 0)
	properties, _, err := unstructured.NestedSlice(t, "properties")
	if err != nil {
		return nil, err
	}
	for _, pro := range properties {
		p, _ := pro.(map[string]interface{})

		name, ok, err := unstructured.NestedString(p, "name")
		if err != nil {
			return nil, err
		}
		if ok {
			value, _, _ := unstructured.NestedString(p, "value")
			_ = unstructured.SetNestedField(v1alpha2Spec, value, name)
		}
	}
	_ = unstructured.SetNestedField(v1alpha2Trait, v1alpha2Spec, "spec")

	return v1alpha2Trait, nil
}
