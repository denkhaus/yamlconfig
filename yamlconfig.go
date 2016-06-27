package yamlconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/globocom/config"
	"github.com/juju/errors"
)

var (
	logger = logrus.New().WithField("pkg", "yamlconfig")
)

type loadDefFn func(conf *YamlConfig)

type YamlConfig struct {
	configFilePath string
}

////////////////////////////////////////////////////////////////////////////////
type ConfigSection struct {
	data interface{}
	mut  sync.RWMutex
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) Unmarshal(target interface{}) error {
	data, ok := m.data.(map[interface{}]interface{})
	if !ok {
		return errors.New("data is no map")
	}

	d := make(map[string]interface{})
	for k, v := range data {
		d[k.(string)] = v
	}

	byt, err := json.Marshal(d)
	if err != nil {
		return errors.Annotate(err, "marshal data")
	}
	if err := json.Unmarshal(byt, target); err != nil {
		return errors.Annotate(err, "unmarshal data")
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) get(key string) (interface{}, error) {
	keys := strings.Split(key, ":")
	m.mut.RLock()
	defer m.mut.RUnlock()

	data, ok := m.data.(map[interface{}]interface{})
	if !ok {
		return nil, errors.New("data is no map")
	}

	conf, ok := data[keys[0]]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}

	for _, k := range keys[1:] {
		conf, ok = conf.(map[interface{}]interface{})[k]
		if !ok {
			return nil, errors.Errorf("key %q not found", key)
		}
	}

	return conf, nil
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetObject(key string) interface{} {
	value, err := m.get(key)
	if err != nil {
		logger.Fatalf("key %s not available", key)

	}

	return value
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetObjectDefault(key string, def interface{}) interface{} {
	value, err := m.get(key)
	if err != nil {
		return def
	}

	return value
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetString(key string) string {
	return m.GetObject(key).(string)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetStringDefault(key string, def string) string {
	value, err := m.get(key)
	if err != nil {
		return def
	}

	return value.(string)
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
func (m *ConfigSection) GetBoolDefault(key string, def bool) bool {
	value, err := m.get(key)
	if err != nil {
		return def
	}

	return value.(bool)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetInt(key string) int {
	return m.GetObject(key).(int)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetIntDefault(key string, def int) int {
	value, err := m.get(key)
	if err != nil {
		return def
	}

	return value.(int)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetFloat64(key string) float64 {
	return m.GetObject(key).(float64)
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetFloat64Default(key string, def float64) float64 {
	value, err := m.get(key)
	if err != nil {
		return def
	}

	return value.(float64)
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

	return -1
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetDurationDefault(key string, def time.Duration) time.Duration {
	value := m.GetDuration(key)
	if value == time.Duration(-1) {
		return def
	}

	return value
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) GetSection(key string) (*ConfigSection, error) {
	data := m.GetObject(key)
	if data == nil {
		return nil, errors.Errorf("data for key '%s' is not available", key)
	}

	return &ConfigSection{data: data}, nil
}

////////////////////////////////////////////////////////////////////////////////
func (m *ConfigSection) MustGetSection(key string) *ConfigSection {
	s, err := m.GetSection(key)
	if err != nil {
		panic(err.Error())
	}
	return s
}

////////////////////////////////////////////////////////////////////////////////

func (m *ConfigSection) GetRaw() interface{} {
	return m.data
}

////////////////////////////////////////////////////////////////////////////////
func (c *YamlConfig) GetConfigSection(key string) (*ConfigSection, error) {
	data, err := config.Get(key)
	if err != nil {
		return nil, errors.Annotate(err, "get data")
	}
	if data == nil {
		return nil, errors.Annotatef(err, "data for key '%s' is not available", key)
	}

	m := ConfigSection{data: data}
	return &m, nil
}

////////////////////////////////////////////////////////////////////////////////
func (c *YamlConfig) SetDefault(key string, value interface{}) {
	if _, err := config.Get(key); err != nil {
		config.Set(key, value)
	}
}

////////////////////////////////////////////////////////////////////////////////
func (c *YamlConfig) confFilePath() (string, error) {
	cnfPath := c.configFilePath

	if cnfPath == "" {
		return "", errors.New("config file path not specified.")
	}

	if !path.IsAbs(cnfPath) {
		if p, err := filepath.Abs(cnfPath); err == nil {
			cnfPath = p
		}
	}

	if _, err := os.Stat(cnfPath); err == nil {
		return cnfPath, nil
	}

	logger.Infof("config file not found at %q, looking in cwd", cnfPath)

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("YamlConfig::Error::Getwd failed: %s", err)
	}

	// look in current directory and make absolute
	filePath := path.Clean(path.Join(wd, cnfPath))
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil
	}

	logger.Infof("config file not found at %q, looking in users home directory", cnfPath)

	// look in current users home directory
	usr, err := user.Current()
	if err != nil {
		return "", errors.Annotate(err, "get current user:")
	}

	cnfPath = path.Clean(path.Join(usr.HomeDir, cnfPath))
	if _, err := os.Stat(cnfPath); err != nil {
		logger.Infof("YamlConfig::Config file in home directory not found. Create new config file at %q", cnfPath)
		if err = config.WriteConfigFile(cnfPath, 0644); err != nil {
			return "", fmt.Errorf("YamlConfig::Write new config::%s", err)
		}
	}

	return cnfPath, nil
}

////////////////////////////////////////////////////////////////////////////////
func (c *YamlConfig) Load(loadDefaults loadDefFn, watchConfig bool) error {
	loadDefaults(c)

	filePath, err := c.confFilePath()
	if err != nil {
		return errors.Annotate(err, "get current file path")
	}

	logger.Infof("load config from path:%q", filePath)
	if watchConfig {
		if err := config.ReadAndWatchConfigFile(filePath); err != nil {
			return errors.Annotate(err, "read and watch config")
		}
	} else {
		if err := config.ReadConfigFile(filePath); err != nil {
			return errors.Annotate(err, "read config")
		}
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
func New(filePath string) *YamlConfig {
	config := YamlConfig{configFilePath: filePath}
	return &config
}
