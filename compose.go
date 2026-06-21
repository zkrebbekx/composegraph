package composegraph

import "gopkg.in/yaml.v3"

// composeFile is the subset of the docker-compose schema this package reads.
// Fields not used for graphing (build args, env, ports, command, ...) are
// intentionally absent.
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	DependsOn   dependsOn `yaml:"depends_on"`
	Networks    stringSet `yaml:"networks"`
	VolumesFrom []string  `yaml:"volumes_from"`
	HealthCheck *struct{} `yaml:"healthcheck"`
}

// dependsOn accepts both compose forms:
//
//	depends_on: [a, b]
//	depends_on: {a: {condition: service_healthy}, b: {}}
type dependsOn map[string]dependsOnEntry

type dependsOnEntry struct {
	Condition string `yaml:"condition"`
}

func (d *dependsOn) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var names []string
		if err := value.Decode(&names); err != nil {
			return err
		}
		*d = make(dependsOn, len(names))
		for _, n := range names {
			(*d)[n] = dependsOnEntry{}
		}
		return nil
	case yaml.MappingNode:
		var m map[string]dependsOnEntry
		if err := value.Decode(&m); err != nil {
			return err
		}
		*d = m
		return nil
	default:
		*d = nil
		return nil
	}
}

// stringSet accepts both compose forms:
//
//	networks: [a, b]
//	networks: {a: {aliases: [...]}, b: null}
type stringSet []string

func (s *stringSet) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var names []string
		if err := value.Decode(&names); err != nil {
			return err
		}
		*s = names
		return nil
	case yaml.MappingNode:
		names := make([]string, 0, len(value.Content)/2)
		for i := 0; i < len(value.Content); i += 2 {
			names = append(names, value.Content[i].Value)
		}
		*s = names
		return nil
	default:
		*s = nil
		return nil
	}
}

// parseCompose decodes a docker-compose YAML document.
func parseCompose(src []byte) (*composeFile, error) {
	var cf composeFile
	if err := yaml.Unmarshal(src, &cf); err != nil {
		return nil, err
	}
	return &cf, nil
}

// mergeComposeFiles parses each document and merges them service-by-service
// in order, base file first — the common docker-compose.yml +
// docker-compose.override.yml pattern. This is a practical union merge, not
// full compose merge semantics: a service present in more than one file has
// its depends_on/networks/volumes_from unioned and its healthcheck replaced
// if the later file sets one; there's no array-replace strategy or deep
// merge of other fields.
func mergeComposeFiles(srcs [][]byte) (*composeFile, error) {
	merged := &composeFile{Services: map[string]composeService{}}
	for _, src := range srcs {
		cf, err := parseCompose(src)
		if err != nil {
			return nil, err
		}
		for name, svc := range cf.Services {
			if existing, ok := merged.Services[name]; ok {
				merged.Services[name] = mergeComposeService(existing, svc)
			} else {
				merged.Services[name] = svc
			}
		}
	}
	return merged, nil
}

func mergeComposeService(base, override composeService) composeService {
	merged := base
	if len(override.DependsOn) > 0 {
		if merged.DependsOn == nil {
			merged.DependsOn = dependsOn{}
		}
		for k, v := range override.DependsOn {
			merged.DependsOn[k] = v
		}
	}
	merged.Networks = unionStrings(merged.Networks, override.Networks)
	merged.VolumesFrom = unionStrings(merged.VolumesFrom, override.VolumesFrom)
	if override.HealthCheck != nil {
		merged.HealthCheck = override.HealthCheck
	}
	return merged
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var out []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
