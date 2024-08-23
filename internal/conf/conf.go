package conf

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"strconv"
)

const osPref = "SD"

const DevelopMode = "devel"

type Config struct {
	// DB connection uri
	// format is `postgresql://user:pass@host:port/db_name`
	DB string `envconfig:"DB"`
	// Cache connection uri
	// It can be redis format or internal
	Cache string `envconfig:"CACHE"`
	// Log level for verbosity
	LogLevel string `envconfig:"LOG_LEVEL"`
	// App port
	Port string `envconfig:"PORT" default:"8000"`
}

func (c *Config) Validate() error {
	p, err := strconv.Atoi(c.Port)
	if err != nil {
		return fmt.Errorf("wront SD_PORT format, should be a number in range 1025:50000")
	}
	if p < 1024 || p > 50000 {
		return fmt.Errorf("wrong port for http server")
	}
	return nil
}

// LoadConf loads configuration from .env file and environment.
// Env variables are preferred.
func LoadConf() (*Config, error) {
	envMap, err := godotenv.Read()
	if err != nil {
		return nil, err
	}

	var c Config
	err = envconfig.Process(osPref, &c)
	if err != nil {
		return nil, err
	}

	mergeConfigs(envMap, &c)

	if err = c.Validate(); err != nil {
		return nil, err
	}

	return &c, nil
}

func mergeConfigs(env map[string]string, c *Config) {
	// TODO: use reflect to automate it
	if c.DB == "" {
		v, ok := env["SD_DB"]
		if ok {
			c.DB = v
		}
	}
	if c.Cache == "" {
		v, ok := env["SD_CACHE"]
		if ok {
			c.Cache = v
		}
	}
	if c.LogLevel == "" {
		v, ok := env["SD_LOG_LEVEL"]
		if ok {
			c.LogLevel = v
		}
	}
	if c.Port == "" {
		v, ok := env["SD_PORT"]
		if ok {
			c.Port = v
		}
	}
}
