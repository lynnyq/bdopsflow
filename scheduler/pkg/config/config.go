package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	configFile string
	values     map[string]interface{}
}

type Options struct {
	ConfigFile string
	Defaults   map[string]string
}

func New(opts ...Options) (*Config, error) {
	cfg := &Config{
		values: make(map[string]interface{}),
	}

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	if opt.ConfigFile != "" {
		cfg.configFile = opt.ConfigFile
	} else {
		cfg.configFile = findConfigFile()
	}

	if cfg.configFile != "" {
		if err := cfg.loadFile(); err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", cfg.configFile, err)
		}
	}

	if opt.Defaults != nil {
		for k, v := range opt.Defaults {
			if _, exists := cfg.values[k]; !exists {
				cfg.values[k] = v
			}
		}
	}

	return cfg, nil
}

func findConfigFile() string {
	possibleFiles := []string{
		"config.yaml",
		"config.yml",
		"./config.yaml",
		"./config.yml",
		"/etc/bdopsflow/config.yaml",
	}

	for _, f := range possibleFiles {
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}

	execPath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(execPath)
		configPath := filepath.Join(dir, "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	return ""
}

func (c *Config) loadFile() error {
	data, err := os.ReadFile(c.configFile)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	c.values = flattenMap("", raw)
	return nil
}

func flattenMap(prefix string, m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			nested := flattenMap(key, val)
			for nk, nv := range nested {
				result[nk] = nv
			}
		default:
			result[key] = val
		}
	}

	return result
}

func (c *Config) GetString(key, defaultVal string) string {
	val := c.getValue(key)
	if val == nil {
		return defaultVal
	}

	switch v := val.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (c *Config) GetInt(key string, defaultVal int) int {
	val := c.getValue(key)
	if val == nil {
		return defaultVal
	}

	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func (c *Config) GetInt32(key string, defaultVal int32) int32 {
	val := c.getValue(key)
	if val == nil {
		return defaultVal
	}

	switch v := val.(type) {
	case int:
		return int32(v)
	case int64:
		return int32(v)
	case float64:
		return int32(v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(i)
		}
	}
	return defaultVal
}

func (c *Config) GetInt64(key string, defaultVal int64) int64 {
	val := c.getValue(key)
	if val == nil {
		return defaultVal
	}

	switch v := val.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}

func (c *Config) GetBool(key string, defaultVal bool) bool {
	val := c.getValue(key)
	if val == nil {
		return defaultVal
	}

	switch v := val.(type) {
	case bool:
		return v
	case string:
		if v == "true" || v == "1" || v == "yes" {
			return true
		}
		if v == "false" || v == "0" || v == "no" {
			return false
		}
	}
	return defaultVal
}

func (c *Config) GetFloat(key string, defaultVal float64) float64 {
	val := c.getValue(key)
	if val == nil {
		return defaultVal
	}

	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

func (c *Config) GetStringMap(key string) map[string]interface{} {
	val := c.getValue(key)
	if val == nil {
		return nil
	}

	if m, ok := val.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func (c *Config) getValue(key string) interface{} {
	if val, ok := c.values[key]; ok {
		return val
	}

	envKey := toEnvKey(key)
	if envVal := os.Getenv(envKey); envVal != "" {
		return envVal
	}

	return nil
}

func toEnvKey(key string) string {
	result := ""
	for i, part := range splitKey(key) {
		if i > 0 {
			result += "_"
		}
		result += toUpperSnakeCase(part)
	}
	return result
}

func splitKey(key string) []string {
	var parts []string
	var current string

	for _, ch := range key {
		if ch == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func toUpperSnakeCase(s string) string {
	result := ""
	for i, ch := range s {
		if ch >= 'A' && ch <= 'Z' && i > 0 {
			result += "_"
		}
		result += string(ch)
	}
	return toUpper(result)
}

func toUpper(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		result[i] = c
	}
	return string(result)
}

func (c *Config) ConfigFile() string {
	return c.configFile
}
