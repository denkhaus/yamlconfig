package yamlconfig

import (
	"fmt"
	"github.com/denkhaus/tcgl/applog"
	"github.com/globocom/config"
	"os"
	"os/user"
	"path"
	"strconv"
	"time"
)

type loadDefFn func(conf *Config)

type Config struct {
	defaults       map[string]interface{}
	configFileName string
}

func NewConfig(fileName string) *Config {
	config := &Config{
		configFileName: fileName,
		defaults:       make(map[string]interface{}),
	}

	return config
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
//
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) ThrowKeyPanic(key string) {
	panic(fmt.Sprintf("config error: key %s not available", key))
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
//
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) ThrowConversionPanic(src interface{}, err error) {
	panic(fmt.Sprintf("config error: cannot convert %v :: error : %s", src, err.Error()))
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
//
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) stringSlice2IntSlice(values []string) []int {

	if values != nil {
		res := make([]int, len(values))
		for n, src := range values {
			if v, err := strconv.Atoi(src); err == nil {
				res[n] = v
			} else {
				c.ThrowConversionPanic(src, err)
			}
		}
		return res
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
// GetInt
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) GetInt(key string) int {

	value, err := config.GetInt(key)
	if err != nil {
		if value, ok := c.defaults[key]; ok {
			return value.(int)
		}
		c.ThrowKeyPanic(key)
	}

	return value
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
// GetIntList
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) GetIntList(key string) []int {

	if value, err := config.GetList(key); err == nil {
		return c.stringSlice2IntSlice(value)
	} else {
		if val, ok := c.defaults[key]; ok {
			return val.([]int)
		} else {
			c.ThrowKeyPanic(key)
		}
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
// GetString
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) GetString(key string) string {

	value, err := config.GetString(key)
	if err != nil {
		if val, ok := c.defaults[key]; ok {
			return val.(string)
		} else {
			c.ThrowKeyPanic(key)
		}
	}

	return value
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
// GetStringList
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) GetStringList(key string) []string {

	value, err := config.GetList(key)
	if err != nil {
		if value, ok := c.defaults[key]; ok {
			return value.([]string)
		} else {
			c.ThrowKeyPanic(key)
		}
	}

	return value
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
// GetDuration
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) GetDuration(key string) time.Duration {

	value, err := config.GetDuration(key)
	if err != nil {
		if val, ok := c.defaults[key]; ok {
			return val.(time.Duration)
		} else {
			c.ThrowKeyPanic(key)
		}
	}

	return value
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
//
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) SetDefault(key string, value interface{}) {
	c.defaults[key] = value
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
//
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) writeDefConfigFile(filePath string) error {

	for key, value := range c.defaults {
		config.Set(key, value)
	}

	applog.Infof("Create new config file at %s", filePath)
	return config.WriteConfigFile(filePath, 0644)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////
// If filePath is defined and the file exists, Load gets the configuration from there, otherwise
// it looks for the filename, specified in NewConfig in the current users home directory. If no file can be found
// the function creates a file with the users default values.
///////////////////////////////////////////////////////////////////////////////////////////////////////
func (c *Config) Load(loadDefaults loadDefFn, filePath string, watchConfig bool) error {

	if len(filePath) == 0 {
		usr, err := user.Current()
		if err != nil {
			return err
		}

		filePath = path.Join(usr.HomeDir, c.configFileName)
	}

	loadDefaults(c)
	if _, err := os.Stat(filePath); err == nil {
		if watchConfig {
			if err := config.ReadAndWatchConfigFile(filePath); err != nil {
				return err
			}
		} else {
			if err := config.ReadConfigFile(filePath); err != nil {
				return err
			}
		}
	} else {
		if err = c.writeDefConfigFile(filePath); err != nil {
			return err
		}
	}

	return nil
}
