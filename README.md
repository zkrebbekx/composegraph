# composegraph

[![CI](https://github.com/zkrebbekx/composegraph/actions/workflows/ci.yml/badge.svg)](https://github.com/zkrebbekx/composegraph/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/zkrebbekx/composegraph.svg)](https://pkg.go.dev/github.com/zkrebbekx/composegraph)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Render a `docker-compose.yml` or Kubernetes manifest's dependency graph to
SVG — in **pure Go**, via [go-mermaid](https://github.com/zkrebbekx/go-mermaid).
No headless browser, no Node.js, no Graphviz. Just a library and a single
static binary.

### ▶ [Try it live in your browser → zkrebbekx.github.io/composegraph](https://zkrebbekx.github.io/composegraph/)

Paste a compose file or a Kubernetes manifest and watch it render instantly —
diagram, raw Mermaid source, and a nodes/edges breakdown, running 100%
client-side (no server, your YAML never leaves the tab).

> The **library** (`github.com/zkrebbekx/composegraph`) is **pure Go with no
> WebAssembly** — nothing you import pulls in a wasm runtime. WebAssembly is
> used *only* by the separate [`playground/`](playground) module, which
> compiles the library to a `GOOS=js` build so it can run in the browser.
> Importing composegraph into your Go program involves no wasm.

## Why

A compose file or a manifest set encodes a real dependency graph
(`depends_on`/`networks`, or Service selectors/Ingress backends/owner
references) that's easy to lose track of once a stack grows past a handful
of resources. `composegraph` turns it into a picture, without shelling out
to anything. The input format is detected automatically.

## Install

Library:

```bash
go get github.com/zkrebbekx/composegraph
```

CLI:

```bash
go install github.com/zkrebbekx/composegraph/cmd/composegraph@latest
```

Homebrew:

```bash
brew install zkrebbekx/tap/composegraph
```

Docker:

```bash
docker run -i --rm -v "$PWD":/work ghcr.io/zkrebbekx/composegraph /work/docker-compose.yml > graph.svg
```

Prebuilt binaries for Linux/macOS/Windows (amd64/arm64) are attached to each
[GitHub release](https://github.com/zkrebbekx/composegraph/releases).

## Usage

```bash
composegraph docker-compose.yml > graph.svg
composegraph -format mmd -o graph.mmd docker-compose.yml   # raw Mermaid source
composegraph -format png -scale 2 -o graph.png docker-compose.yml
composegraph a/docker-compose.yml b/docker-compose.yml     # batch: writes a.svg, b.svg
echo "$(cat docker-compose.yml)" | composegraph > graph.svg

# Kubernetes: a manifest, or several -- one file's worth of `---`-separated
# documents, or several files concatenated, work the same way.
composegraph deployment.yaml > graph.svg
cat namespace.yaml deployment.yaml service.yaml ingress.yaml | composegraph > graph.svg

# Compose base + override: merge instead of batch.
composegraph -merge -o graph.svg docker-compose.yml docker-compose.override.yml
```

Library:

```go
package main

import (
	"os"

	"github.com/zkrebbekx/composegraph"
)

func main() {
	src, _ := os.ReadFile("docker-compose.yml")
	svg, err := composegraph.Render(src)
	if err != nil {
		panic(err)
	}
	os.WriteFile("graph.svg", svg, 0o644)
}
```

`Render` accepts the same functional options as `go-mermaid.Render` (theme,
spacing, ...): `composegraph.Render(src, mermaid.WithTheme(mermaid.Dark))`.

Need just the Mermaid source (e.g. to paste into another tool)?
`composegraph.ToMermaid(src)` returns the flowchart text without rendering it.

Merging compose files programmatically: `composegraph.ToMermaidMerged(srcs)`
/ `composegraph.RenderMerged(srcs, opts...)` take a `[][]byte`. Kubernetes
manifests don't need an equivalent — `ToMermaid`/`Render` already read a
`---`-separated multi-document stream, so concatenating files is enough.

## How it maps

### docker-compose

| compose | diagram |
| --- | --- |
| each service | a node — stadium-shaped (`([name])`) if it has a `healthcheck`, rectangle otherwise |
| `depends_on` | a directed edge; a non-default condition (`service_healthy`, `service_completed_successfully`) becomes an edge label |
| `volumes_from` | a directed edge labeled `volumes` |
| `networks` | services sharing a network are grouped into a subgraph titled after it (a service in several networks is grouped under the first, alphabetically) |

If the compose file never uses `networks:` at all, the diagram is left flat
(no subgraphs) — there'd be nothing to cluster by. The same applies to
Kubernetes namespaces below.

`-merge` (or `composegraph.ToMermaidMerged`/`RenderMerged`) folds several
compose files into one graph — the `docker-compose.yml` +
`docker-compose.override.yml` pattern. It's a practical union merge, not
full compose merge semantics: a service defined in more than one file has
its `depends_on`/`networks`/`volumes_from` unioned and its `healthcheck`
replaced if a later file sets one. There's no deep merge of other fields
or array-replace strategy.

### Kubernetes

| manifest | diagram |
| --- | --- |
| Deployment / StatefulSet / DaemonSet | a node — stadium-shaped if any container declares a `readinessProbe`/`livenessProbe`, rectangle otherwise |
| Service | a rounded node |
| Ingress | a hexagon node |
| ConfigMap / Secret | a cylinder node |
| ReplicaSet / Pod / Job | a rectangle node — these only ever appear via `ownerReferences` (see below); a hand-authored manifest set won't have them, but a `kubectl get all -o yaml` dump will |
| `ownerReferences` | a directed edge, owner → owned (e.g. Deployment → ReplicaSet → Pod) |
| Service `spec.selector` matching a workload's `spec.template.metadata.labels` | a directed edge, Service → workload |
| Ingress backend (`backend.service.name`, or the legacy `backend.serviceName`) matching a Service name | a directed edge, Ingress → Service, labeled with the route path |
| a workload's `envFrom`/`env[].valueFrom`/`volumes` referencing a ConfigMap or Secret by name | a directed edge, workload → ConfigMap/Secret, labeled `env` |
| `metadata.namespace` | resources sharing a namespace are grouped into a subgraph titled after it |

Label selectors are matched as a plain conjunction of key/value equality
(`matchExpressions` aren't evaluated) — the same practical-subset spirit as
everything else here. RBAC, Namespace, and other scaffolding kinds are
parsed (so `ownerReferences` pointing at them still resolve) but never
drawn.

## Roadmap

- [x] `docker-compose.yml`: services, `depends_on`, `networks`, `volumes_from`
- [x] `docker-compose.override.yml` multi-file merge (`-merge`)
- [x] Kubernetes manifests: Deployment/StatefulSet/DaemonSet, Service,
      Ingress, ConfigMap/Secret, ReplicaSet/Pod/Job via `ownerReferences`
- [x] SVG / Mermaid source / PNG output, batch mode, stdin
- [x] Distribution: prebuilt binaries, Homebrew cask, ghcr.io Docker image

## Develop

```sh
make test
make lint
make build
```

## License

MIT
