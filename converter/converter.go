package converter

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var k8sClient client.Client
var ctx = context.Background()
var err error

func init() {
	k8sClient, err = client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		klog.Info("init k8sClinet error: ", err)
	}
}

// ConvertAppConfig is a function that converts v1alpha1 types to v1alpha2 types of ApplicationConfigurations
func ConvertAppConfig(Object *unstructured.Unstructured, toVersion string) (*unstructured.Unstructured, metav1.Status) {
	klog.V(2).Info("converting crd")

	convertedObject := Object.DeepCopy()
	fromVersion := Object.GetAPIVersion()

	if toVersion == fromVersion {
		return nil, statusErrorWithMessage("conversion from a version to itself should not call the webhook: %s", toVersion)
	}

	converterPlugin := ExamplePlugin{}

	switch Object.GetAPIVersion() {
	case "core.oam.dev/v1alpha1":
		switch toVersion {
		case "core.oam.dev/v1alpha2":
			components, _, err := unstructured.NestedSlice(convertedObject.Object, "spec", "components")
			if err != nil {
				return nil, statusErrorWithMessage("get spec.components error: %v", err)
			}

			v1alpha2Components := make([]interface{}, 0)
			for _, comp := range components {
				// convert appconfig.spec.components field and return a v1alpha2 component CR
				c, compCR, err := converterPlugin.ConvertComponent(comp)
				if err != nil {
					return nil, statusErrorWithMessage("convert components error: %v", err)
				}
				// If there is no component in the cluster, apply the v1alpha2 component
				compObject := client.ObjectKey{Name: compCR.Name, Namespace: compCR.Namespace}
				if err = k8sClient.Get(ctx, compObject, &compCR); err != nil {
					if ok := apierrors.IsNotFound(err); ok {
						if err = k8sClient.Create(ctx, &compCR); err != nil {
							klog.Info("create v1alpha2 component error: ", err)
						}
						klog.Info("create v1alpha2 component successfully, name: ", compCR.Name, " namespace: ", compCR.Namespace, " APIVersion: ", compCR.APIVersion)
					}
				}

				traits, _, _ := unstructured.NestedSlice(c, "traits")

				v1alpha2Traits := make([]interface{}, 0)
				for _, trait := range traits {
					// convert appconfig.spec.components.traits field
					tr, err := converterPlugin.ConvertTrait(trait)
					if err != nil {
						return nil, statusErrorWithMessage("convert trait error: %v", err)
					}

					tempTrait := make(map[string]interface{}, 0)
					_ = unstructured.SetNestedField(tempTrait, map[string]interface{}(tr), "trait")
					v1alpha2Traits = append(v1alpha2Traits, tempTrait)
				}

				// Remove the old field and nest the new field
				unstructured.RemoveNestedField(c, "traits")
				err = unstructured.SetNestedSlice(c, v1alpha2Traits, "traits")
				if err != nil {
					return nil, statusErrorWithMessage("set component.traits error: %v", err)
				}

				v1alpha2Components = append(v1alpha2Components, map[string]interface{}(c))
			}

			// Remove the old field and nest the new field
			unstructured.RemoveNestedField(convertedObject.Object, "spec", "components")
			err = unstructured.SetNestedSlice(convertedObject.Object, v1alpha2Components, "spec", "components")
			if err != nil {
				klog.Info("set spec.components err: ", err)
			}
		default:
			return nil, statusErrorWithMessage("unexpected conversion version %q", toVersion)
		}
	case "core.oam.dev/v1alpha2":
		switch toVersion {
		case "core.oam.dev/v1alpha1":
			//
		default:
			return nil, statusErrorWithMessage("unexpected conversion version %q", toVersion)
		}
	default:
		return nil, statusErrorWithMessage("unexpected conversion version %q", fromVersion)
	}

	return convertedObject, statusSucceed()
}
