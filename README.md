# oam-crd-migration

A tool to help you migrate OAM CRDs from v1alpha1 to v1alpha2.

This migration tool is primarily a conversion webhook like admission webhook. This webhook
handles the `ConversionReview` requests sent by the API servers, and sends back conversion
results wrapped in `ConversionResponse` . The specific conversion logic can be customized by the
user.

More details see [this](https://github.com/crossplane/oam-kubernetes-runtime/issues/108).

# Migration Solutions

In [this doc](docs/migration_solutions.md) we introduced all possible solutions we have thought through to do migration in general mechanism.

This tool implements the first one which aligns with upstream and provides a self-contained solution.

# What does it include?
- [x] [crd conversion webhook](https://github.com/kubernetes/kubernetes/tree/master/test/images/agnhost)
- [x] [a golang script](https://github.com/elastic/cloud-on-k8s/issues/2196) to remove old versions from CRD `status.storedVersions`

# For developers
[converter/framework.go:](converter/framework.go)
- Functions such as `Server` define how to handle `ConversionReview` requests and responses and generally do not need to be changed.
- `convertFunc` is the user defined function for any conversion. The code in this file is a template that can be use for any CR conversion given this function. Or users can customize the input and output to be a specific type of CR.
    ```
    type convertFunc func(Object *unstructured.Unstructured, version string) (*unstructured.Unstructured, metav1.Status)
    ```
- The `doConversionV1` and `doConversionV1beta1` functions also need to be modified if the user uses a specific type of CR as the input/output of `convertFunc` . Change the `cr` variable type to the CR desired by the user.
    ```
    func doConversionV1(convertRequest *v1.ConversionRequest, convert convertFunc) *v1.ConversionResponse {
        var convertedObjects []runtime.RawExtension
        for _, obj := range convertRequest.Objects {
            cr := unstructured.Unstructured{}
            if err := cr.UnmarshalJSON(obj.Raw); err != nil {
                ...
            }
            klog.Info("get storage object successfully, its version:", cr.GetAPIVersion(), ", its name:", cr.GetName())
            convertedCR, status := convert(&cr, convertRequest.DesiredAPIVersion)
        ...
    ```
[converter/converter.go:](converter/converter.go)
- `ConvertAppConfig` is an implementation of `convertFunc` , and the specific conversion logic is a modification of unstructured nested structure, replacing the old field description of v1alpha1 with the new field description of v1alpha2.
    ```
    func ConvertAppConfig(Object *unstructured.Unstructured, toVersion string) (*unstructured.Unstructured, metav1.Status)
    ```
    
[converter/plugin.go:](converter/plugin.go)
- The interface defines a collection of conversion methods for the `Components` and `Traits` fields in ApplicationConfiguration. Users can customize the conversion logic by implementing methods.
    ```
    type Converter interface {
        ConvertComponent(v1alpha1Component) (v1alpha2Component, v1alpha2.Component, error)
        ConvertTrait(v1alpha1Trait) (v1alpha2Trait, error)
    }
    ```
- Variable definition: Because the ApplicationConfiguration in this example is an unstructured structure, variables are defined as `interface{}` or `map[string]interface{}` for the convenience of obtaining and modifying specific fields in the nested structure.
    ```
    type v1alpha1Component interface{}
    
    type v1alpha2Component map[string]interface{}
    
    type v1alpha1Trait interface{}
    
    type v1alpha2Trait map[string]interface{}
    ```

# User guide for appconfig examples
## Pre-requisites
- Clusters with old versions of CRD
    ```
    kubectl kustomize ./crd/bases/ | kubectl apply -f -
    
    kubectl apply -f crd/appconfig_v1alpha1_example.yaml
    ```
- Because webhook is deployed in default namespace in this demo, permissions should be assigned.
    ```
    kubectl apply -f crd/role-binding.yaml
    ```
## The conversion process
- Create secret for ssl certificates
    ```
    curl -sfL https://raw.githubusercontent.com/crossplane/oam-kubernetes-runtime/master/hack/ssl/ssl.sh | bash -s oam-crd-conversion default
    
    kubectl create secret generic webhook-server-cert --from-file=tls.key=./oam-crd-conversion.key --from-file=tls.crt=./oam-crd-conversion.pem
    ```
- Create CA Bundle info and inject into the CRD definition
    ```
    caValue=`kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}'`
    
    sed -i 's/${CA_BUNDLE}/'"$caValue"'/g' ./crd/patches/crd_conversion_applicationconfigurations.yaml
    ```
- Build image and deploy a deployment and a service for webhook
    ```
    docker build -t example:v0.1 .

    kubectl apply -f deploy/webhook.yaml
    ```
- Patch new versions and conversion strategy to CRD
    ```
    kubectl get crd applicationconfigurations.core.oam.dev -o yaml >> ./crd/patches/temp.yaml
  
    kubectl kustomize ./crd/patches | kubectl apply -f -
    ```
- Verify that the old and new version objects are available
    ```
    # kubectl describe applicationconfigurations complete-app
    
    Name:         complete-app
    Namespace:    default
    Labels:       <none>
    Annotations:  API Version:  core.oam.dev/v1alpha2
    Kind:         ApplicationConfiguration
    ...
      Traits:
        Trait:
          API Version:  core.oam.dev/v1alpha2
          Kind:         RollOutTrait
          Metadata:
            Name:  rollout
          Spec:
            Auto:               true
            Batch Interval:     5
            Batches:            2
            Canary Replicas:    0
            Instance Interval:  1
    
    # kubectl describe applicationconfigurations.v1alpha1.core.oam.dev complete-app
    
    Name:         complete-app
    Namespace:    default
    Labels:       <none>
    Annotations:  API Version:  core.oam.dev/v1alpha1
    Kind:         ApplicationConfiguration
    ...
      Traits:
        Name:  rollout
        Properties:
          Name:   canaryReplicas
          Value:  0
          Name:   batches
          Value:  2
          Name:   batchInterval
          Value:  5
          Name:   instanceInterval
          Value:  1
          Name:   auto
          Value:  true
    ```
## Update existing objects
Here we use kube-storage-version-migrator as an example, you can write Go scripts instead.
- Run the storage Version migrator
    ```
    git clone https://github.com/kubernetes-sigs/kube-storage-version-migrator
  
    sed -i 's/kube-system/default/g' ./Makefile
  
    make local-manifests
  
    sed -i '1,5d' ./manifests.local/namespace-rbac.yaml
  
    pushd manifests.local && kubectl apply -k ./ && popd
    ```
- Verify the migration is "SUCCEEDED"
    ```
    kubectl get storageversionmigrations -o=custom-columns=NAME:.spec.resource.resource,STATUS:.status.conditions[0].type
  
    NAME                       STATUS
    ...                        ...
    applicationconfigurations  SUCCEEDED
    ...                        ...
    ```
## Remove old versions
- Run the golang script that removes old versions from CRD `status.storedVersions` field
    ```
    go run remove/remove.go
  
    updated applicationconfigurations.core.oam.dev CRD status storedVersions: [v1alpha2]
    ```
- Verify the script runs successfully
    ```
    kubectl describe crd applicationconfigurations.core.oam.dev
  
    Name:         applicationconfigurations.core.oam.dev
    Namespace:    
    ...
      Stored Versions:
        v1alpha2
    Events:  <none>
    ```
- Remove the old version from the CustomResourceDefinition spec.versions list
    ```
    kubectl get crd applicationconfigurations.core.oam.dev -o yaml >> ./crd/complete/temp.yaml
  
    kubectl kustomize ./crd/complete | kubectl apply -f -
    ```
