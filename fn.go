package main

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/crossplane/function-template-go/input/v1beta1"
	"google.golang.org/protobuf/encoding/protojson"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/yaml"
	"strings"
)

const (
	composedNameProviderConfig  = "function-tee-provider-kubernetes-config"
	composedNameOutputConfigMap = "function-tee-output-configmap"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	f.log.Info("Running Function", "tag", req.GetMeta().GetTag())

	// This creates a new response to the supplied request. Note that Functions
	// are run in a pipeline! Other Functions may have run before this one. If
	// they did, response.To will copy their desired state from req to rsp. Be
	// sure to pass through any desired state your Function is not concerned
	// with unmodified.
	rsp := response.To(req, response.DefaultTTL)

	// Input is supplied by the author of a Composition when they choose to run
	// your Function. Input is arbitrary, except that it must be a KRM-like
	// object. Supporting input is also optional - if you don't need to you can
	// delete this, and delete the input directory.
	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	// TODO: Should we default to in which namespace the Function is running instead?
	cmNs := "crossplane-system"
	if in.ConfigMapNamespace != "" {
		cmNs = in.ConfigMapNamespace
	}

	yReq, err := extractRequestAsYaml(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot extract request as yaml"))
		return rsp, nil
	}

	xr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get observed composed resources from %T", req))
		return rsp, nil
	}

	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get desired composite resources from %T", req))
		return rsp, nil
	}

	addDesiredTo(desired, strings.ToLower(fmt.Sprintf("%s.%s", xr.Resource.GetKind(), xr.Resource.GetName())), cmNs, yReq)

	err = response.SetDesiredComposedResources(rsp, desired)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources in %T", rsp))
		return rsp, nil
	}

	response.Normal(rsp, "Successful run")

	return rsp, nil
}

func extractRequestAsYaml(req *fnv1beta1.RunFunctionRequest) (string, error) {
	// No protoyaml.Marshal and seems not planned: https://github.com/golang/protobuf/issues/1519#issuecomment-1416897454
	jReq, err := protojson.Marshal(req)
	if err != nil {
		return "", errors.Wrap(err, "cannot marshal request from proto to json")
	}

	var mReq map[string]interface{}
	err = json.Unmarshal(jReq, &mReq)
	if err != nil {
		return "", errors.Wrap(err, "cannot unmarshal json to map[string]interface{}")
	}

	// Do some cleanup of the request
	paved := fieldpath.Pave(mReq)
	var ors map[string]map[string]interface{}
	if err = paved.GetValueInto("observed.resources", &ors); err != nil && !fieldpath.IsNotFound(err) {
		return "", errors.Wrap(err, "cannot get observed resources from request")
	}
	for k := range ors {
		if k == composedNameOutputConfigMap || k == composedNameProviderConfig {
			// We don't want to pollute the observed resources with our
			//  ProviderConfig and ConfigMap
			delete(ors, k)
		}
		// TODO: Make this optional via input
		if err = fieldpath.Pave(ors[k]).DeleteField("resource.metadata.managedFields"); err != nil && !fieldpath.IsNotFound(err) {
			return "", errors.Wrap(err, "cannot delete managedFields from observed resources")
		}
	}
	if err = paved.SetValue("observed.resources", ors); err != nil {
		return "", errors.Wrap(err, "cannot set observed resources in request")
	}
	// Also clean up managed fields from the composite resource
	if err = paved.DeleteField("observed.composite.resource.metadata.managedFields"); err != nil && !fieldpath.IsNotFound(err) {
		return "", errors.Wrap(err, "cannot delete managedFields from observed composite resource")
	}

	yReq, err := yaml.Marshal(mReq)
	if err != nil {
		return "", errors.Wrap(err, "cannot marshal map to yaml")
	}

	return string(yReq), nil
}

func addDesiredTo(existing map[resource.Name]*resource.DesiredComposed, name, namespace, requestYaml string) {

	existing[composedNameProviderConfig] = &resource.DesiredComposed{
		Resource: &composed.Unstructured{
			Unstructured: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubernetes.crossplane.io/v1alpha1",
					"kind":       "ProviderConfig",
					"metadata": map[string]interface{}{
						// TODO: We are violating one rule of the Composition Function
						//  contract here: we are setting the name of the ProviderConfig
						//  to a fixed value. This is because we need to refer to it
						//  in our configmap Object.
						//  We need this https://github.com/crossplane/crossplane-runtime/issues/319
						"name": "function-tee",
					},
					"spec": map[string]interface{}{
						"credentials": map[string]interface{}{
							"source": "InjectedIdentity",
						},
					},
				},
			},
		},
		Ready: resource.ReadyTrue,
	}

	existing[composedNameOutputConfigMap] = &resource.DesiredComposed{
		Resource: &composed.Unstructured{
			Unstructured: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubernetes.crossplane.io/v1alpha1",
					"kind":       "Object",
					"spec": map[string]interface{}{
						"providerConfigRef": map[string]interface{}{
							"name": "function-tee",
						},
						"forProvider": map[string]interface{}{
							"manifest": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]interface{}{
									"name":      name,
									"namespace": namespace,
								},
								"data": map[string]interface{}{
									"request": requestYaml,
								},
							},
						},
					},
				},
			},
		},
		// Note: Do I have to set this? I would prefer the ready status to be
		//  inferred from Object status
		Ready: resource.ReadyTrue,
	}
}
