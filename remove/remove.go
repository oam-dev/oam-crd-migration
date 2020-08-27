package main

import (
	"context"
	"fmt"

	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
	//  create k8s client
	ctx := context.Background()
	cfg, err := config.GetConfig()
	client, err := apiextension.NewForConfig(cfg)
	if err != nil {
		fmt.Println(err)
		return
	}

	updateStatus(client, ctx, "applicationconfigurations.core.oam.dev")
}

// updateStatus remove v1alpha1 from CRD status
func updateStatus(client *apiextension.Clientset, ctx context.Context, gvk string) {
	// retrieve CRD
	crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, gvk, v1.GetOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	// remove v1alpha1 from its status
	oldStoredVersions := crd.Status.StoredVersions
	newStoredVersions := make([]string, 0, len(oldStoredVersions))
	for _, stored := range oldStoredVersions {
		if stored != "v1alpha1" {
			newStoredVersions = append(newStoredVersions, stored)
		}
	}
	crd.Status.StoredVersions = newStoredVersions
	// update the status sub-resource
	crd, err = client.ApiextensionsV1().CustomResourceDefinitions().UpdateStatus(ctx, crd, v1.UpdateOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("updated", gvk, "CRD status storedVersions:", crd.Status.StoredVersions)
}
