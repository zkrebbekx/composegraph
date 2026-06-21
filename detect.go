package composegraph

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// detectFormat sniffs an input document as either a docker-compose file
// (a top-level `services:` key) or a Kubernetes manifest (a top-level
// `apiVersion:`/`kind:` pair on its first YAML document — manifests may
// have more than one `---`-separated document, but only the first is
// needed to tell the formats apart, since a compose file is always a
// single document).
func detectFormat(src []byte) (string, error) {
	var first map[string]any
	dec := yaml.NewDecoder(bytes.NewReader(src))
	if err := dec.Decode(&first); err != nil {
		return "", fmt.Errorf("composegraph: %w", err)
	}
	if _, ok := first["services"]; ok {
		return "compose", nil
	}
	if _, hasKind := first["kind"]; hasKind {
		if _, hasAPIVersion := first["apiVersion"]; hasAPIVersion {
			return "k8s", nil
		}
	}
	return "", fmt.Errorf("composegraph: unrecognized input (expected a docker-compose file with a top-level `services:` key, or a Kubernetes manifest with `apiVersion`/`kind`)")
}
