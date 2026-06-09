# ServiceLayer AppCat Pipeline Package Emission Renderer (Slapper)

It slaps your service into AppCat form!

## Getting started

Check out the example to AppSlappify an existing helm chart.

TODO: more docs and a proper getting started, once we have something.

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
