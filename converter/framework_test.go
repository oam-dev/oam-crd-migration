package converter

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/crossplane/oam-kubernetes-runtime/apis/core/v1alpha2"
)

func TestConverter(t *testing.T) {
	cases := []struct {
		apiVersion     string
		contentType    string
		expected400Err string
	}{
		{
			apiVersion:     "apiextensions.k8s.io/v1beta1",
			contentType:    "application/json",
			expected400Err: "json parse error",
		},
		{
			apiVersion:  "apiextensions.k8s.io/v1beta1",
			contentType: "application/yaml",
		},
		{
			apiVersion:     "apiextensions.k8s.io/v1",
			contentType:    "application/json",
			expected400Err: "json parse error",
		},
		{
			apiVersion:  "apiextensions.k8s.io/v1",
			contentType: "application/yaml",
		},
	}
	sampleObjTemplate := `kind: ConversionReview
apiVersion: %s
request:
  uid: 0000-0000-0000-0000
  desiredAPIVersion: core.oam.dev/v1alpha2
  objects:
    - apiVersion: core.oam.dev/v1alpha1
      kind: ApplicationConfiguration
      metadata:
        name: test-appconfig
      spec:
        components:
          - componentName: test-component
            instanceName: demo
            parameterValues:
              - name: description
                value: demo
            traits:
              - name: rollout
                properties:
                  - name: test
                    value: "0"
`
	for _, tc := range cases {
		t.Run(tc.apiVersion+" "+tc.contentType, func(t *testing.T) {
			sampleObj := fmt.Sprintf(sampleObjTemplate, tc.apiVersion)
			// First try json, it should fail as the data is yaml
			response := httptest.NewRecorder()
			request, err := http.NewRequest("POST", "/convert", strings.NewReader(sampleObj))
			if err != nil {
				t.Error(err)
			}
			request.Header.Add("Content-Type", tc.contentType)
			ServeAppConfigConvert(response, request)
			convertReview := apiextensionsv1.ConversionReview{}
			scheme := runtime.NewScheme()
			if len(tc.expected400Err) > 0 {
				body := response.Body.Bytes()
				if !bytes.Contains(body, []byte(tc.expected400Err)) {
					t.Errorf("expected to fail on '%s', but it failed with: %s", tc.expected400Err, string(body))
				}
				return
			}

			yamlSerializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true})
			if _, _, err := yamlSerializer.Decode(response.Body.Bytes(), nil, &convertReview); err != nil {
				t.Errorf("cannot decode data: \n %v\n Error: %v", response.Body, err)
			}
			if convertReview.Response.Result.Status != v1.StatusSuccess {
				t.Errorf("cr conversion failed: %v", convertReview.Response)
			}
			convertedObj := unstructured.Unstructured{}
			if _, _, err := yamlSerializer.Decode(convertReview.Response.ConvertedObjects[0].Raw, nil, &convertedObj); err != nil {
				t.Error(err)
			}
			if e, a := "core.oam.dev/v1alpha2", convertedObj.GetAPIVersion(); e != a {
				t.Errorf("expected= %v, actual= %v", e, a)
			}
		})
	}
	// Delete unnecessary component that is generated during the test
	testObjectKey := client.ObjectKey{Name: "test-component", Namespace: "default"}
	var testCompCR v1alpha2.Component
	_ = k8sClient.Get(ctx, testObjectKey, &testCompCR)
	_ = k8sClient.Delete(ctx, &testCompCR)
}
