package composegraph

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// k8sResource is one decoded Kubernetes manifest document. Raw holds the
// full document for kind-specific field extraction — the API surface
// across object kinds varies too much to justify typed structs for a
// practical subset.
type k8sResource struct {
	Kind      string
	Name      string
	Namespace string
	Labels    map[string]string
	OwnerRefs []ownerRef
	Raw       map[string]any
}

type ownerRef struct {
	Kind, Name string
}

// parseK8sResources decodes a (possibly multi-document, `---`-separated)
// Kubernetes manifest stream into resources. Documents without a `kind`
// are skipped.
func parseK8sResources(src []byte) ([]k8sResource, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var resources []k8sResource
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if doc == nil {
			continue
		}
		kind, _ := doc["kind"].(string)
		if kind == "" {
			continue
		}
		meta, _ := doc["metadata"].(map[string]any)
		name, _ := meta["name"].(string)
		namespace, _ := meta["namespace"].(string)
		resources = append(resources, k8sResource{
			Kind:      kind,
			Name:      name,
			Namespace: namespace,
			Labels:    toStringMap(meta["labels"]),
			OwnerRefs: ownerRefs(meta["ownerReferences"]),
			Raw:       doc,
		})
	}
	return resources, nil
}

func ownerRefs(v any) []ownerRef {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	var refs []ownerRef
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := m["kind"].(string)
		name, _ := m["name"].(string)
		if kind != "" && name != "" {
			refs = append(refs, ownerRef{Kind: kind, Name: name})
		}
	}
	return refs
}

func toStringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, vv := range m {
		if s, ok := vv.(string); ok {
			out[k] = s
		}
	}
	return out
}

// getPath walks a chain of nested map[string]any keys, returning false if
// any step is missing or not itself a map.
func getPath(m map[string]any, path ...string) (any, bool) {
	var cur any = m
	for _, p := range path {
		cm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = cm[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func getStringPath(m map[string]any, path ...string) (string, bool) {
	v, ok := getPath(m, path...)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func getMapPath(m map[string]any, path ...string) (map[string]any, bool) {
	v, ok := getPath(m, path...)
	if !ok {
		return nil, false
	}
	mm, ok := v.(map[string]any)
	return mm, ok
}

// getStringMapPath reads a string-valued map at path (e.g. a selector or
// label map), returning nil if the path is absent or not such a map.
func getStringMapPath(m map[string]any, path ...string) map[string]string {
	v, ok := getMapPath(m, path...)
	if !ok {
		return nil
	}
	return toStringMap(v)
}

func getSlicePath(m map[string]any, path ...string) ([]any, bool) {
	v, ok := getPath(m, path...)
	if !ok {
		return nil, false
	}
	s, ok := v.([]any)
	return s, ok
}

// asMaps filters a []any down to its map[string]any elements.
func asMaps(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func resourceKey(r k8sResource) string {
	return fmt.Sprintf("%s/%s/%s", namespaceOrDefault(r.Namespace), r.Kind, r.Name)
}

func namespaceOrDefault(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}
