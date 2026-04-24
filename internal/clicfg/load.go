package clicfg

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v2"
)

// Load resolves the effective CLI configuration using this precedence:
//  1. defaults
//  2. YAML file (explicit path, OTELKIT_CONFIG env, or discovery chain)
//  3. selected profile overlay (OTELKIT_PROFILE)
//  4. OTELKIT_* env var overrides
//
// path == "" triggers discovery. An explicit path that points to a missing
// file is an error; missing files during discovery are silent.
func Load(path, profile string) (*Config, error) {
	cfg := Defaults()

	resolved, explicit, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	if resolved != "" {
		data, err := os.ReadFile(resolved)
		if err != nil {
			if explicit {
				return nil, fmt.Errorf("read config %s: %w", resolved, err)
			}
		} else {
			if err := applyYAML(cfg, data); err != nil {
				return nil, err
			}
			cfg.sourcePath = resolved
		}
	}

	if err := applyProfile(cfg, profile); err != nil {
		return nil, err
	}

	applyEnvOverrides(cfg)
	expandStrings(cfg)

	return cfg, nil
}

func resolvePath(explicit string) (path string, wasExplicit bool, err error) {
	if explicit != "" {
		return explicit, true, nil
	}
	if v := os.Getenv("OTELKIT_CONFIG"); v != "" {
		return v, true, nil
	}
	candidates := []string{"otelkit.yaml", "otelkit.yml"}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "otelkit", "config.yaml"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "otelkit", "config.yaml"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs, false, nil
		}
	}
	return "", false, nil
}

func applyYAML(cfg *Config, data []byte) error {
	warnings, err := decode(data, cfg)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		log.Printf("otelkit: %s", w)
	}
	return nil
}

func applyProfile(cfg *Config, name string) error {
	if name == "" {
		name = os.Getenv("OTELKIT_PROFILE")
	}
	if name == "" {
		return nil
	}
	raw, ok := cfg.Profiles[name]
	if !ok {
		names := make([]string, 0, len(cfg.Profiles))
		for k := range cfg.Profiles {
			names = append(names, k)
		}
		return fmt.Errorf("profile %q not found (available: %v)", name, names)
	}

	currentBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	var current map[string]any
	if err := yaml.Unmarshal(currentBytes, &current); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	delete(current, "profiles")

	merged := Merge(current, map[string]any(raw))
	mergedBytes, err := yaml.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshal merged: %w", err)
	}

	newCfg := *cfg
	newCfg.Profiles = nil
	if err := yaml.Unmarshal(mergedBytes, &newCfg); err != nil {
		return fmt.Errorf("decode merged: %w", err)
	}
	newCfg.Profiles = cfg.Profiles
	newCfg.sourcePath = cfg.sourcePath
	newCfg.activeProfile = name
	*cfg = newCfg
	return nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("OTELKIT_GRPC_ADDR"); v != "" {
		cfg.Receiver.GRPCAddr = v
	}
	if v := os.Getenv("OTELKIT_STORAGE_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Storage.Enabled = b
		}
	}
	if v := os.Getenv("OTELKIT_STORAGE_PATH"); v != "" {
		cfg.Storage.Path = v
	}
	if v := os.Getenv("OTELKIT_TUI_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.TUI.Enabled = b
		}
	}
}

func expandStrings(cfg *Config) {
	cfg.Storage.Path = Expand(cfg.Storage.Path)
	for i, f := range cfg.Fanout {
		f.Endpoint = Expand(f.Endpoint)
		f.Headers = f.Headers.ExpandHeaders()
		cfg.Fanout[i] = f
	}
}

func decode(data []byte, target any) (warnings []string, err error) {
	if err := yaml.Unmarshal(data, target); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("yaml raw parse: %w", err)
	}

	tv := reflect.ValueOf(target)
	if tv.Kind() != reflect.Ptr || tv.IsNil() {
		return nil, fmt.Errorf("target must be non-nil pointer")
	}
	tt := tv.Elem().Type()
	warnings = collectUnknownKeys("", raw, tt)
	return warnings, nil
}

func collectUnknownKeys(prefix string, raw any, t reflect.Type) []string {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	known := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := strings.Split(f.Tag.Get("yaml"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}
		known[tag] = f.Type
	}

	var warnings []string
	for k, v := range m {
		fieldType, ok := known[k]
		if !ok {
			full := k
			if prefix != "" {
				full = prefix + "." + k
			}
			warnings = append(warnings, fmt.Sprintf("unknown key: %s", full))
			continue
		}
		// Recurse into nested structs and map values whose element is a struct.
		switch fieldType.Kind() {
		case reflect.Struct:
			child := prefix
			if child != "" {
				child += "."
			}
			child += k
			warnings = append(warnings, collectUnknownKeys(child, v, fieldType)...)
		case reflect.Map:
			if fieldType.Elem().Kind() == reflect.Struct {
				if childMap, ok := v.(map[string]any); ok {
					for ck, cv := range childMap {
						childPrefix := prefix
						if childPrefix != "" {
							childPrefix += "."
						}
						childPrefix += k + "." + ck
						warnings = append(warnings, collectUnknownKeys(childPrefix, cv, fieldType.Elem())...)
					}
				}
			}
		case reflect.Slice:
			if fieldType.Elem().Kind() == reflect.Struct {
				if childList, ok := v.([]any); ok {
					for i, cv := range childList {
						childPrefix := prefix
						if childPrefix != "" {
							childPrefix += "."
						}
						childPrefix += fmt.Sprintf("%s[%d]", k, i)
						warnings = append(warnings, collectUnknownKeys(childPrefix, cv, fieldType.Elem())...)
					}
				}
			}
		}
	}
	return warnings
}
