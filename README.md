# ServiceLayer AppCat Pipeline Package Emission Renderer (Slapper)

It slaps your service into AppCat form!

## What?!

Slapper takes an opinionated service description and converts it into Crossplane compositions and XRDs.
These manifests can then either be applied directly to a cluster, or be turned into a xpkg.

A service bundle specifies what a service needs to be running.
Additionally, it also describes an API for the end-user to spawn an instance of the service.

The service bundle consists of:

- The service's base manifests like a helm chart or some plain manifests
- It's possible to pass default values to the chart and map fields from the XRD to the values as well
- Additional configuration in an implementation agnostic way, for example backups, network, or maintenance
- Custom steps to define additional application specific deployment logic

Important is, that apart from the base manifest and custom logic, all steps in the service bundle only describe an intent.
The implementation of the pipeline steps comes from the stdlib.
Stdlibs contain the specific implementation for each of the pipeline steps.
Different stdlib can be used depending on the customer or platform.

This allows for abstracting the service specifics from the platform and customer specifics.
A service maintainer doesn't have to ask: "do I need an ingress or gateway manifest?" they only need to specify
how the service should get exposed, the rest is handled by the stdlib.

## Getting started

On a fresh kindev cluster:

- Install Crossplane
- Install CloudNativePG
- Install the necessary providers and functions
- Generate the example
- Apply it

### 1. Install Crossplane

```sh
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update
helm install crossplane crossplane-stable/crossplane \
  --namespace crossplane-system --create-namespace \
```

Wait until the deployment is ready:

```sh
kubectl -n crossplane-system rollout status deploy/crossplane
```

### 2. Install CloudNativePG

The PostgreSQL example depends on the CloudNativePG operator.

```sh
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm repo update
helm install cnpg cnpg/cloudnative-pg \
  --namespace cnpg-system --create-namespace
```

Wait until the operator is ready:

```sh
kubectl -n cnpg-system rollout status deploy/cnpg-cloudnative-pg
```

### 3. Render the example bundle

```sh
make build
./slap examples/servicebundle.yaml -p pkg/converter/stdlib/testdata
```

This populates `xpkg/` with `xrd.yaml` and `composition.yaml`.

### 4. Install dependencies + generated package

Install all dependencies.

```sh
kubectl apply -f examples/bootstrap/
kubectl apply -f xpkg/xrd.yaml -f xpkg/composition.yaml
```

> RBAC for provider-helm will be cluster-admin, don't use this in
> production.

Wait for the packages to become healthy:

```sh
kubectl wait --for=condition=Healthy provider/provider-helm --timeout=5m
kubectl wait --for=condition=Healthy function/function-kcl --timeout=5m
kubectl wait --for=condition=Healthy function/function-python --timeout=5m
```

### 5. Create an instance

```sh
kubectl apply -f examples/xvshnpostgresql.yaml
kubectl get xvshnpostgresql my-pg -w
```

## Stdlib resolution

The pipeline step renderers, the framework-wide XRD schema fragments and the
Crossplane Configuration `dependsOn` list are sourced from an external
**stdlib** instead of being baked into the binary. The CLI supports three
resolution modes:

### Default — OCI artifact from `meta.stdlib`

The bundle's `meta.stdlib` field is treated as an OCI reference. The artifact
is pulled and cached by digest under `$XDG_CACHE_HOME/slapper/stdlib`.
Subsequent runs with the same digest skip the network.

```yaml
meta:
  stdlib: ghcr.io/vshn/servicebundle-stdlib:v0.1.0
```

```sh
slap examples/servicebundle.yaml
```

### `--stdlib-path` — local directory

Point at an unpacked stdlib on disk. Useful for stdlib development.
`meta.stdlib` is ignored.

```sh
slap --stdlib-path ./my-stdlib examples/servicebundle.yaml
```

### `--no-stdlib` — in-tree dummies, debug only

Skip stdlib resolution entirely. Step renderers fall back to in-tree dummies
and no `crossplane.yaml` Configuration meta is emitted. If `meta.stdlib` is
set, a warning is logged. Not for production output.

```sh
slap --no-stdlib examples/servicebundle.yaml
```

### Other flags

- `--stdlib-cache-dir` — override the digest-keyed cache location.
- `--output` — change the package output directory (default `xpkg`).
