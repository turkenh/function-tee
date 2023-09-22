# function-tee

A [Crossplane] Composition Function similar to [tee] in Unix. It outputs the
input request it receives into a configmap, and then passes it through to the
next Function in the pipeline.

Crossplane sends the following in a [RunFunctionRequest]:

1. The observed state of the XR, and any existing composed resources.
1. The desired state of the XR, and any existing composed resources.
1. The input to your Function (if any), as specified in the Composition.

**Example:**

<details>

<summary>composition.yaml</summary>

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: xnopresources.nop.example.org
spec:
  compositeTypeRef:
    apiVersion: nop.example.org/v1alpha1
    kind: XNopResource
  mode: Pipeline
  pipeline:
  - step: be-a-dummy
    functionRef:
      name: function-dummy
    input:
      apiVersion: dummy.fn.crossplane.io/v1beta1
      kind: Response
      # This is a YAML-serialized RunFunctionResponse. function-dummy will
      # overlay the desired state on any that was passed into it.
      response:
        desired:
          composite:
            resource:
              status:
                coolerField: "I'M COOLER!"
          resources:
            nop-resource-1:
              resource:
                apiVersion: nop.crossplane.io/v1alpha1
                kind: NopResource
                spec:
                  forProvider:
                    conditionAfter:
                    - conditionType: Ready
                      conditionStatus: "False"
                      time: 0s
                    - conditionType: Ready
                      conditionStatus: "True"
                      time: 10s
              ready: READY_TRUE
        results:
         - severity: SEVERITY_NORMAL
           message: "I am doing a compose!"
  - step: tee
    functionRef:
      name: function-tee
```

</details>

It will create a ConfigMap outputting what it got as input request in the pipeline:

```shell
# For an XR name "apiextensions-composition-functions-k5mtg" of kind "xnopresources"
kubectl -n crossplane-system get cm xnopresource.apiextensions-composition-functions-k5mtg -o jsonpath={.data.request}
```

<details>

<summary>output</summary>

```yaml
desired:
  composite:
    resource:
      status:
        coolerField: I'M COOLER!
  resources:
    nop-resource-1:
      ready: READY_TRUE
      resource:
        apiVersion: nop.crossplane.io/v1alpha1
        kind: NopResource
        spec:
          forProvider:
            conditionAfter:
            - conditionStatus: "False"
              conditionType: Ready
              time: 0s
            - conditionStatus: "True"
              conditionType: Ready
              time: 10s
observed:
  composite:
    resource:
      apiVersion: nop.example.org/v1alpha1
      kind: XNopResource
      metadata:
        creationTimestamp: "2023-09-22T09:40:54Z"
        finalizers:
        - composite.apiextensions.crossplane.io
        generateName: apiextensions-composition-functions-
        generation: 4
        labels:
          crossplane.io/claim-name: apiextensions-composition-functions
          crossplane.io/claim-namespace: default
          crossplane.io/composite: apiextensions-composition-functions-k5mtg
        name: apiextensions-composition-functions-k5mtg
        resourceVersion: "27323"
        uid: ea581c53-9edd-453d-91ab-39869e7f7aff
      spec:
        claimRef:
          apiVersion: nop.example.org/v1alpha1
          kind: NopResource
          name: apiextensions-composition-functions
          namespace: default
        compositionRef:
          name: xnopresources.nop.example.org
        compositionRevisionRef:
          name: xnopresources.nop.example.org-e84f2b1
        compositionUpdatePolicy: Automatic
        coolField: I'm cool!
        resourceRefs:
        - apiVersion: kubernetes.crossplane.io/v1alpha1
          kind: Object
          name: apiextensions-composition-functions-k5mtg-wqbnb
        - apiVersion: kubernetes.crossplane.io/v1alpha1
          kind: ProviderConfig
          name: function-tee
        - apiVersion: nop.crossplane.io/v1alpha1
          kind: NopResource
          name: apiextensions-composition-functions-k5mtg-kwnmg
      status:
        conditions:
        - lastTransitionTime: "2023-09-22T09:41:07Z"
          reason: ReconcileSuccess
          status: "True"
          type: Synced
        - lastTransitionTime: "2023-09-22T09:41:07Z"
          reason: Available
          status: "True"
          type: Ready
        coolerField: I'M COOLER!
  resources:
    nop-resource-1:
      resource:
        apiVersion: nop.crossplane.io/v1alpha1
        kind: NopResource
        metadata:
          annotations:
            crossplane.io/composition-resource-name: nop-resource-1
            crossplane.io/external-name: apiextensions-composition-functions-k5mtg-kwnmg
          creationTimestamp: "2023-09-22T09:41:07Z"
          finalizers:
          - finalizer.managedresource.crossplane.io
          generateName: apiextensions-composition-functions-k5mtg-
          generation: 2
          labels:
            crossplane.io/claim-name: apiextensions-composition-functions
            crossplane.io/claim-namespace: default
            crossplane.io/composite: apiextensions-composition-functions-k5mtg
          name: apiextensions-composition-functions-k5mtg-kwnmg
          ownerReferences:
          - apiVersion: nop.example.org/v1alpha1
            blockOwnerDeletion: true
            controller: true
            kind: XNopResource
            name: apiextensions-composition-functions-k5mtg
            uid: ea581c53-9edd-453d-91ab-39869e7f7aff
          resourceVersion: "27328"
          uid: cacd495b-ffcc-4b24-bd67-16bc0078e05c
        spec:
          deletionPolicy: Delete
          forProvider:
            conditionAfter:
            - conditionStatus: "True"
              conditionType: Ready
              time: 10s
            - conditionStatus: "False"
              conditionType: Ready
              time: 0s
          providerConfigRef:
            name: default
        status:
          atProvider: {}
          conditions:
          - lastTransitionTime: "2023-09-22T09:41:07Z"
            reason: ReconcileSuccess
            status: "True"
            type: Synced
```

</details>

## Installing

```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-tee
spec:
  package: turkenh/function-tee:v0.1.0
```

## Building

```shell
# Run code generation - see input/generate.go
$ go generate ./...

# Run tests
$ go test -cover ./...
?       github.com/crossplane/function-tee/input/v1beta1      [no test files]
ok      github.com/crossplane/function-tee    0.006s  coverage: 25.8% of statements

# Lint the code
$ docker run --rm -v $(pwd):/app -v ~/.cache/golangci-lint/v1.54.2:/root/.cache -w /app golangci/golangci-lint:v1.54.2 golangci-lint run

# Build a Docker image - see Dockerfile
$ docker build .
```

This Function can be pushed to any Docker registry. To push to xpkg.upbound.io
use `docker push` and `docker-credential-up` from
https://github.com/upbound/up/.

## Known Issues

It is just a prototype for now, not ready for production use.
- [ ] Improve naming of ConfigMap. It might conflict with other kinds of different API group.
- [ ] Optionally include/exclude connection details
- [ ] Optionally include `managedFields`
- [ ] Optionally include/exclude observed or desired state

[Crossplane]: https://crossplane.io
[tee]: https://en.wikipedia.org/wiki/Tee_(command)
[RunFunctionRequest]: https://github.com/crossplane/function-sdk-go/blob/a4ada4f934f6f8d3f9018581199c6c71e0343d13/proto/v1beta1/run_function.proto#L36

