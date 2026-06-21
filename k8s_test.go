package composegraph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zkrebbekx/composegraph"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFormatDetection(t *testing.T) {
	Convey("Given a Kubernetes manifest", t, func() {
		src := []byte(`
apiVersion: v1
kind: Service
metadata:
  name: api
spec:
  selector:
    app: api
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then it is detected and rendered as Kubernetes, not compose", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "(api)")
			})
		})
	})

	Convey("Given input that's neither a compose file nor a Kubernetes manifest", t, func() {
		src := []byte("foo: bar\n")

		Convey("When converted to Mermaid", func() {
			_, err := composegraph.ToMermaid(src)

			Convey("Then it reports the format as unrecognized", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "unrecognized input")
			})
		})
	})
}

func TestK8sShapes(t *testing.T) {
	Convey("Given a Deployment with a readiness probe, a Service, an Ingress, and a ConfigMap", t, func() {
		src := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: shop
spec:
  template:
    metadata:
      labels: {app: api}
    spec:
      containers:
        - name: api
          readinessProbe: {}
---
apiVersion: v1
kind: Service
metadata:
  name: api-svc
  namespace: shop
spec:
  selector: {app: api}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api-ingress
  namespace: shop
spec:
  rules:
    - http:
        paths:
          - path: /api
            backend: {service: {name: api-svc}}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: api-config
  namespace: shop
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then each kind maps to its shape", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "([api])")         // probed workload: stadium
				So(mmd, ShouldContainSubstring, "(api-svc)")       // Service: rounded
				So(mmd, ShouldContainSubstring, "{{api-ingress}}") // Ingress: hexagon
				So(mmd, ShouldContainSubstring, "[(api-config)]")  // ConfigMap: cylinder
			})

			Convey("Then the Service selects the workload and the Ingress routes to the Service", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "n4 --> n2\n") // Service (n4) -> Deployment (n2)
				So(mmd, ShouldContainSubstring, "n3 -->|/api| n4\n")
			})
		})
	})

	Convey("Given a workload with no probe", t, func() {
		src := []byte(`
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: db
spec:
  template:
    metadata: {labels: {app: db}}
    spec:
      containers: [{name: db}]
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then it renders as a plain rectangle", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "[db]")
				So(mmd, ShouldNotContainSubstring, "([db])")
			})
		})
	})
}

func TestK8sOwnerReferences(t *testing.T) {
	Convey("Given a Deployment owning a ReplicaSet owning a Pod", t, func() {
		src := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: shop
---
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: app-7f8c9
  namespace: shop
  ownerReferences: [{kind: Deployment, name: app}]
---
apiVersion: v1
kind: Pod
metadata:
  name: app-7f8c9-x2z9k
  namespace: shop
  ownerReferences: [{kind: ReplicaSet, name: app-7f8c9}]
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then the ownership chain becomes edges, owner pointing at owned", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "n1 --> n3\n") // Deployment -> ReplicaSet
				So(mmd, ShouldContainSubstring, "n3 --> n2\n") // ReplicaSet -> Pod
			})
		})
	})
}

func TestK8sConfigAndSecretRefs(t *testing.T) {
	Convey("Given a Deployment referencing a ConfigMap via envFrom and a Secret via env.valueFrom", t, func() {
		src := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: shop
spec:
  template:
    metadata: {labels: {app: api}}
    spec:
      containers:
        - name: api
          envFrom:
            - configMapRef: {name: api-config}
          env:
            - name: DB_PASSWORD
              valueFrom: {secretKeyRef: {name: api-secret, key: password}}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: api-config, namespace: shop}
---
apiVersion: v1
kind: Secret
metadata: {name: api-secret, namespace: shop}
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then both references become env-labeled edges", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "|env|")
			})
		})
	})
}

func TestK8sNamespaceGrouping(t *testing.T) {
	Convey("Given resources in two distinct namespaces", t, func() {
		src := []byte(`
apiVersion: v1
kind: Service
metadata: {name: a, namespace: ns1}
spec: {selector: {app: a}}
---
apiVersion: v1
kind: Service
metadata: {name: b, namespace: ns2}
spec: {selector: {app: b}}
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then each namespace becomes its own subgraph", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldContainSubstring, "subgraph net_ns1 [ns1]")
				So(mmd, ShouldContainSubstring, "subgraph net_ns2 [ns2]")
			})
		})
	})

	Convey("Given every resource in the same (unset) namespace", t, func() {
		src := []byte(`
apiVersion: v1
kind: Service
metadata: {name: a}
spec: {selector: {app: a}}
`)
		Convey("When converted to Mermaid", func() {
			mmd, err := composegraph.ToMermaid(src)

			Convey("Then there is nothing to cluster by, so it stays flat", func() {
				So(err, ShouldBeNil)
				So(mmd, ShouldNotContainSubstring, "subgraph")
			})
		})
	})
}

func TestK8sRealFixture(t *testing.T) {
	Convey("Given the hand-written k8s_shop.yaml fixture", t, func() {
		src, err := os.ReadFile(filepath.Join("testdata", "k8s_shop.yaml"))
		So(err, ShouldBeNil)

		Convey("When rendered to SVG", func() {
			svg, err := composegraph.Render(src)

			Convey("Then it renders without error", func() {
				So(err, ShouldBeNil)
				So(string(svg), ShouldContainSubstring, "<svg")
			})
		})
	})
}
