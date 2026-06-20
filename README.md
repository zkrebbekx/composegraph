# composegraph

[![CI](https://github.com/zkrebbekx/composegraph/actions/workflows/ci.yml/badge.svg)](https://github.com/zkrebbekx/composegraph/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/zkrebbekx/composegraph.svg)](https://pkg.go.dev/github.com/zkrebbekx/composegraph)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Render a `docker-compose.yml` service dependency graph to SVG — in **pure
Go**, via [go-mermaid](https://github.com/zkrebbekx/go-mermaid). No headless
browser, no Node.js, no Graphviz. Just a library and a single static binary.

## Why

`docker-compose.yml` encodes a real dependency graph (`depends_on`,
`networks`, `volumes_from`) that's easy to lose track of once a stack grows
past a handful of services. `composegraph` turns it into a picture, without
shelling out to anything.

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

## How it maps

| compose | diagram |
| --- | --- |
| each service | a node — stadium-shaped (`([name])`) if it has a `healthcheck`, rectangle otherwise |
| `depends_on` | a directed edge; a non-default condition (`service_healthy`, `service_completed_successfully`) becomes an edge label |
| `volumes_from` | a directed edge labeled `volumes` |
| `networks` | services sharing a network are grouped into a subgraph titled after it (a service in several networks is grouped under the first, alphabetically) |

If the compose file never uses `networks:` at all, the diagram is left flat
(no subgraphs) — there'd be nothing to cluster by.

## Roadmap

- [x] `docker-compose.yml`: services, `depends_on`, `networks`, `volumes_from`
- [x] SVG / Mermaid source / PNG output, batch mode, stdin
- [x] Distribution: prebuilt binaries, Homebrew cask, ghcr.io Docker image
- [ ] Kubernetes manifests: Deployment/Service/Ingress/StatefulSet, edges
      from `ownerReferences` and Service-selector → Deployment-label matching,
      Ingress → Service → Deployment chains
- [ ] `docker-compose.override.yml` multi-file merge

## Develop

```sh
make test
make lint
make build
```

## License

MIT
