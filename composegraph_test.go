package composegraph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zkrebbekx/composegraph"

	. "github.com/smartystreets/goconvey/convey"
)

func TestToMermaidBasics(t *testing.T) {
	Convey("Given a compose file with two services and no depends_on", t, func() {
		src := []byte(`
services:
  api:
    image: api
  db:
    image: db
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then both services appear as plain rectangle nodes, ungrouped", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "api[api]")
				So(mmd, ShouldContainSubstring, "db[db]")
				So(mmd, ShouldNotContainSubstring, "subgraph")
			})
		})
	})

	Convey("Given a service depending on another with the default condition", t, func() {
		src := []byte(`
services:
  api:
    image: api
    depends_on:
      - db
  db:
    image: db
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then it is a plain unlabeled edge", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "api --> db\n")
			})
		})
	})

	Convey("Given a service depending on another with a non-default condition", t, func() {
		src := []byte(`
services:
  api:
    image: api
    depends_on:
      db:
        condition: service_healthy
  db:
    image: db
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then the edge is labeled with the condition, service_ prefix stripped", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "api -->|healthy| db")
			})
		})
	})

	Convey("Given services sharing a network", t, func() {
		src := []byte(`
services:
  api:
    image: api
    networks: [back]
  db:
    image: db
    networks: [back]
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then both are clustered in a subgraph titled after the network", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "subgraph net_back [back]")
				So(mmd, ShouldContainSubstring, "end")
			})
		})
	})

	Convey("Given a service with a healthcheck", t, func() {
		src := []byte(`
services:
  db:
    image: db
    healthcheck:
      test: ["CMD", "pg_isready"]
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then it renders as a stadium-shaped node", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "db([db])")
			})
		})
	})

	Convey("Given a service using volumes_from", t, func() {
		src := []byte(`
services:
  app:
    image: app
    volumes_from:
      - data
  data:
    image: data
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then the edge is labeled volumes", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "app -->|volumes| data")
			})
		})
	})

	Convey("Given a service name containing characters unsafe for a bare Mermaid ID", t, func() {
		src := []byte(`
services:
  fake-gcs.server:
    image: x
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then the node ID is sanitized but the original name is kept as the label", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "fake_gcs_server[fake-gcs.server]")
			})
		})
	})

	Convey("Given malformed YAML", t, func() {
		src := []byte("services: [this is not a map")

		Convey("When converted to Mermaid", func() {
			_, err := composegraph.ToMermaid(src)

			Convey("Then it returns an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestRenderProducesSVG(t *testing.T) {
	Convey("Given a simple compose file", t, func() {
		src := []byte(`
services:
  api:
    image: api
    depends_on: [db]
  db:
    image: db
`)
		Convey("When rendered", func() {
			svg, err := composegraph.Render(src)

			Convey("Then it produces valid SVG output", func() {
				So(err, ShouldBeNil)
				So(string(svg), ShouldContainSubstring, "<svg")
			})
		})
	})
}

// Real-world fixtures, copied into testdata/ from the user's own repos —
// the diagram doesn't need to be pixel-perfect, but composegraph must not
// choke on what people actually write.
var realFixtures = []struct {
	file     string
	services []string
}{
	{"godump.yml", []string{"clickhouse", "fake-gcs-server", "godump"}},
	{"go-coffeeshop.yml", []string{"postgres", "rabbitmq", "proxy", "product", "counter", "barista", "kitchen", "web"}},
	{"monolith-microservice-shop.yml", nil},
	{"cosmo.yml", nil},
	{"opentelemetry-demo.yml", nil},
}

func TestRealFixtures(t *testing.T) {
	for _, fx := range realFixtures {
		fx := fx
		Convey("Given the real-world compose file "+fx.file, t, func() {
			src, err := os.ReadFile(filepath.Join("testdata", fx.file))
			So(err, ShouldBeNil)

			Convey("When rendered to SVG", func() {
				svg, err := composegraph.Render(src)

				Convey("Then it renders without error", func() {
					So(err, ShouldBeNil)
					So(string(svg), ShouldContainSubstring, "<svg")
				})
			})

			Convey("When converted to Mermaid", func() {
				mmd, err := composegraph.ToMermaid(src)

				Convey("Then every known service appears as a node", func() {
					So(err, ShouldBeNil)
					for _, svc := range fx.services {
						So(mmd, ShouldContainSubstring, svc)
					}
				})
			})
		})
	}
}
