package composegraph

import "sort"

// workloadKinds are the resource kinds treated as Pod-template-bearing
// workloads (selector matching, probes, ConfigMap/Secret refs all read
// from the same spec.template shape).
var workloadKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
}

// graphableKinds are the resource kinds that become nodes. RBAC,
// Namespace, and similar scaffolding kinds are parsed (for ownerReference
// resolution) but never drawn. ReplicaSet/Pod/Job aren't workloadKinds —
// they don't get selector/probe/ConfigMap-ref edges — but they're the
// kinds that actually carry ownerReferences in a `kubectl get all -o yaml`
// dump, so they're graphable to make the ownership chain
// (Deployment → ReplicaSet → Pod) visible.
var graphableKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"ReplicaSet":  true,
	"Pod":         true,
	"Job":         true,
	"Service":     true,
	"Ingress":     true,
	"ConfigMap":   true,
	"Secret":      true,
}

// buildK8sGraph converts decoded Kubernetes resources into a [Graph]:
// nodes are workloads/Services/Ingresses/ConfigMaps/Secrets; edges come
// from ownerReferences, Service selector → workload label matching, and
// Ingress backend → Service name matching, and workload → ConfigMap/Secret
// env/volume references. Resources sharing a namespace are grouped into a
// subgraph if any resource declares one explicitly.
func buildK8sGraph(resources []k8sResource) *Graph {
	sort.Slice(resources, func(i, j int) bool {
		a, b := resources[i], resources[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})

	anyExplicitNamespace := false
	var nodeResources []k8sResource
	byKey := map[string]k8sResource{}
	for _, r := range resources {
		if r.Namespace != "" {
			anyExplicitNamespace = true
		}
		byKey[resourceKey(r)] = r
		if graphableKinds[r.Kind] {
			nodeResources = append(nodeResources, r)
		}
	}

	g := &Graph{}
	for _, r := range nodeResources {
		node := Node{Key: resourceKey(r), Name: r.Name, Shape: shapeFor(r)}
		if anyExplicitNamespace {
			node.Group = namespaceOrDefault(r.Namespace)
		}
		g.Nodes = append(g.Nodes, node)
	}

	for _, r := range nodeResources {
		for _, ref := range r.OwnerRefs {
			ownerKey := namespaceOrDefault(r.Namespace) + "/" + ref.Kind + "/" + ref.Name
			if owner, ok := byKey[ownerKey]; ok && graphableKinds[owner.Kind] {
				g.Edges = append(g.Edges, Edge{From: ownerKey, To: resourceKey(r)})
			}
		}
	}

	services := filterKind(nodeResources, "Service")
	workloads := filterWorkloads(nodeResources)
	for _, svc := range services {
		selector := getStringMapPath(svc.Raw, "spec", "selector")
		if len(selector) == 0 {
			continue
		}
		for _, w := range workloads {
			if w.Namespace != svc.Namespace {
				continue
			}
			labels := getStringMapPath(w.Raw, "spec", "template", "metadata", "labels")
			if labelsMatch(selector, labels) {
				g.Edges = append(g.Edges, Edge{From: resourceKey(svc), To: resourceKey(w)})
			}
		}
	}

	for _, ing := range filterKind(nodeResources, "Ingress") {
		for _, backend := range ingressBackends(ing.Raw) {
			for _, svc := range services {
				if svc.Namespace == ing.Namespace && svc.Name == backend.serviceName {
					g.Edges = append(g.Edges, Edge{From: resourceKey(ing), To: resourceKey(svc), Label: backend.path})
				}
			}
		}
	}

	configsAndSecrets := append(filterKind(nodeResources, "ConfigMap"), filterKind(nodeResources, "Secret")...)
	for _, w := range workloads {
		refs := configMapAndSecretRefs(w.Raw)
		for _, ref := range refs {
			for _, cs := range configsAndSecrets {
				if cs.Namespace == w.Namespace && cs.Kind == ref.kind && cs.Name == ref.name {
					g.Edges = append(g.Edges, Edge{From: resourceKey(w), To: resourceKey(cs), Label: "env"})
				}
			}
		}
	}

	return g
}

func shapeFor(r k8sResource) string {
	switch r.Kind {
	case "Service":
		return "rounded"
	case "Ingress":
		return "hexagon"
	case "ConfigMap", "Secret":
		return "cylinder"
	default:
		if workloadKinds[r.Kind] && hasProbe(r.Raw) {
			return "stadium"
		}
		return ""
	}
}

func filterKind(resources []k8sResource, kind string) []k8sResource {
	var out []k8sResource
	for _, r := range resources {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}

func filterWorkloads(resources []k8sResource) []k8sResource {
	var out []k8sResource
	for _, r := range resources {
		if workloadKinds[r.Kind] {
			out = append(out, r)
		}
	}
	return out
}

// labelsMatch reports whether every key/value in selector is present in
// labels (a Kubernetes label selector is a conjunction of equality
// matches for this practical subset — matchExpressions are not handled).
func labelsMatch(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func hasProbe(spec map[string]any) bool {
	containers, _ := getSlicePath(spec, "spec", "template", "spec", "containers")
	for _, c := range asMaps(containers) {
		if _, ok := c["readinessProbe"]; ok {
			return true
		}
		if _, ok := c["livenessProbe"]; ok {
			return true
		}
	}
	return false
}

type ingressBackend struct {
	serviceName, path string
}

// ingressBackends reads spec.rules[].http.paths[].backend, supporting
// both the current networking.k8s.io/v1 shape (backend.service.name) and
// the legacy extensions/v1beta1 shape (backend.serviceName).
func ingressBackends(ing map[string]any) []ingressBackend {
	rules, _ := getSlicePath(ing, "spec", "rules")
	var out []ingressBackend
	for _, rule := range asMaps(rules) {
		paths, _ := getSlicePath(rule, "http", "paths")
		for _, p := range asMaps(paths) {
			path, _ := p["path"].(string)
			if name, ok := getStringPath(p, "backend", "service", "name"); ok {
				out = append(out, ingressBackend{serviceName: name, path: path})
				continue
			}
			if name, ok := getStringPath(p, "backend", "serviceName"); ok {
				out = append(out, ingressBackend{serviceName: name, path: path})
			}
		}
	}
	return out
}

type configRef struct {
	kind, name string
}

// configMapAndSecretRefs reads every ConfigMap/Secret a workload's pod
// template references: envFrom, env[].valueFrom, and volumes.
func configMapAndSecretRefs(workload map[string]any) []configRef {
	var refs []configRef
	containers, _ := getSlicePath(workload, "spec", "template", "spec", "containers")
	for _, c := range asMaps(containers) {
		for _, ef := range asMaps(toSlice(c["envFrom"])) {
			if name, ok := getStringPath(ef, "configMapRef", "name"); ok {
				refs = append(refs, configRef{kind: "ConfigMap", name: name})
			}
			if name, ok := getStringPath(ef, "secretRef", "name"); ok {
				refs = append(refs, configRef{kind: "Secret", name: name})
			}
		}
		for _, e := range asMaps(toSlice(c["env"])) {
			if name, ok := getStringPath(e, "valueFrom", "configMapKeyRef", "name"); ok {
				refs = append(refs, configRef{kind: "ConfigMap", name: name})
			}
			if name, ok := getStringPath(e, "valueFrom", "secretKeyRef", "name"); ok {
				refs = append(refs, configRef{kind: "Secret", name: name})
			}
		}
	}
	volumes, _ := getSlicePath(workload, "spec", "template", "spec", "volumes")
	for _, v := range asMaps(volumes) {
		if name, ok := getStringPath(v, "configMap", "name"); ok {
			refs = append(refs, configRef{kind: "ConfigMap", name: name})
		}
		if name, ok := getStringPath(v, "secret", "secretName"); ok {
			refs = append(refs, configRef{kind: "Secret", name: name})
		}
	}
	return refs
}

func toSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
