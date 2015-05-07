package yamlconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/denkhaus/tcgl/applog"
	"github.com/globocom/config"
)

type loadDefFn func(conf *YamlConfig)

type YamlConfig struct {
	configFileName string
}

////////////////////////////////////////////////////////////////////////////////
type ConfigSection struct {
	data map[interface{}]interface{}
	mut  sync.RWMutex
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) Unmarshal(target interface{}) error {
	byt, err := json.Marshal(m.data)
	if err != nil {
		fmt.Errorf("YamlConfig::Error::Unmarshal::%s", err)
	}

	return json.Unmarshal(byt, target)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) get(key string) (interface{}, error) {
	keys := strings.Split(key, ":")
	m.mut.RLock()
	defer m.mut.RUnlock()
	conf, ok := m.data[keys[0]]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	for _, k := range keys[1:] {
		conf, ok = conf.(map[interface{}]interface{})[k]
		if !ok {
			return nil, fmt.Errorf("key %q not found", key)
		}
	}

	return conf, nil
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetObject(key string) interface{} {
	value, err := m.get(key)
	if err != nil {
		panic(fmt.Sprintf("YamlConfig::key %s not available", key))
	}

	return value
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetString(key string) string {
	return m.GetObject(key).(string)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetStringList(key string) []string {
	value := m.GetObject(key)

	switch value.(type) {
	case []interface{}:
		v := value.([]interface{})
		result := make([]string, len(v))
		for i, item := range v {
			switch item.(type) {
			case int:
				result[i] = strconv.Itoa(item.(int))
			case bool:
				result[i] = strconv.FormatBool(item.(bool))
			case float64:
				result[i] = strconv.FormatFloat(item.(float64), 'f', -1, 64)
			case string:
				result[i] = item.(string)
			default:
				result[i] = fmt.Sprintf("%v", item)
			}
		}
		return result
	case []string:
		return value.([]string)
	}

	return []string{}
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetBool(key string) bool {
	return m.GetObject(key).(bool)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetInt(key string) int {
	return m.GetObject(key).(int)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetFloat64(key string) float64 {
	return m.GetObject(key).(float64)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetDuration(key string) time.Duration {
	value := m.GetObject(key)

	switch value.(type) {
	case int:
		return time.Duration(value.(int))
	case float64:
		return time.Duration(value.(float64))
	case string:
		if value, err := time.ParseDuration(value.(string)); err == nil {
			return value
		}
	}

	return 0
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetRaw() map[interface{}]interface{} {
	return m.data
}

////////////////////////////////////////////////////////////////////////////////
func (c *YamlConfig) GetConfigSection(key string) (*ConfigSection, error) {
	data, err := config.Get(key)
	if err != nil {
		return nil, fmt.Errorf("YamlConfig::key %s not available", key)
	}
	m := ConfigSection{data: data.(map[interface{}]interface{})}
	return &m, nil
}

////////////////////////////////////////////////////////////////////////////////
func (c *YamlConfig) SetDefault(key string, value interface{}) {
	if _, err := config.Get(key); err != nil {
		config.Set(key, value)
	}
}

////////////////////////////////////////////////////////////////////////////////
func (c *YamlConfig) GetCurrentFilePath() (string, error) {
	if c.configFileName == "" {
		fmt.Errorf("YamlConfig::Error::config filename not specified.")
	}

	// look in current directory and make absolute
	if _, err := os.Stat(c.configFileName); err == nil {
		if !path.IsAbs(c.configFileName) {
			wd, err := os.Getwd()
			if err != nil {
				fmt.Errorf("YamlConfig::Error::Getwd failed: %s", err)
			}
			return path.Clean(path.Join(wd, c.configFileName)), nil
		}
		return c.configFileName, nil
	}

	// look in current users home directory
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("YamlConfig::get current user::%s", err)
	}

	filePath := path.Clean(path.Join(usr.HomeDir, c.configFileName))
	if _, err := os.Stat(filePath); err != nil {
		applog.Infof("YamlConfig::create new config file at %s", filePath)
		if err = config.WriteConfigFile(filePath, 0644); err != nil {
			return "", fmt.Errorf("YamlConfig::Write new config::%s", err)
		}
	}

	return filePath, nil
}

////////////////////////////////////////////////////////////////////////////////
func (c *YamlConfig) Load(loadDefaults loadDefFn, watchConfig bool) error {
	loadDefaults(c)

	filePath, err := c.GetCurrentFilePath()
	if err != nil {
		return fmt.Errorf("YamlConfig::Load::%s", err)
	}

	if watchConfig {
		if err := config.ReadAndWatchConfigFile(filePath); err != nil {
			return err
		}
	} else {
		if err := config.ReadConfigFile(filePath); err != nil {
			return err
		}
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
func New(fileName string) *YamlConfig {
	config := YamlConfig{configFileName: fileName}
	return &config
}
