package config

import (
	"errors"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	_ "log"
)

type (
	Config struct {
		Server    server
		JWT       jwt
		Ratelimit ratelimit
	}
	server struct {
		HostName string
		Port     int
	}

	jwt struct {
		Secret      string
		SecretBytes []byte
	}

	ratelimit struct {
		RequestLimit int

		// Window length in seconds
		WindowLength int
	}
)

func (cfg *Config) Validate() error {
	if err := cfg.Server.Validate(); err != nil {
		return err
	}

	if err := cfg.JWT.Validate(); err != nil {
		return err
	}

	if err := cfg.Ratelimit.Validate(); err != nil {
		return err
	}

	return nil
}

func (s *server) Validate() error {
	if s.Port <= 0 || s.Port > 65535 {
		return errors.New("server.Port needs to be between 1 and 65535")
	}
	return nil
}

func (j *jwt) Validate() error {
	if len(j.Secret) < 16 {
		return errors.New("jwt.Secret needs to be at least 16 chars long")
	}

	return nil
}

func (r *ratelimit) Validate() error {
	if r.RequestLimit <= 0 {
		return errors.New("ratelimit.RequestLimit needs to be greater than 0")
	}

	if r.WindowLength <= 0 {
		return errors.New("ratelimit.WindowLength needs to be greater than 0")
	}

	return nil
}

var AppConfig *Config

func (cfg *Config) String() string {

	ret := "Config\n"
	ret += "Server:\n"
	ret += "\tHostName: " + cfg.Server.HostName + "\n"
	ret += "\tPort: " + strconv.Itoa(cfg.Server.Port) + "\n"
	ret += "JWT:\n"
	//	ret += "\tSecret: " + strings.Repeat("*", len(cfg.JWT.SecretBytes)) + "\n"
	ret += "\tSecret: " + strings.Repeat("*", 16) + "\n"
	ret += "Ratelimit:\n"
	ret += "\tRequestLimit: " + strconv.Itoa(cfg.Ratelimit.RequestLimit) + "\n"
	ret += "\tWindowLength: " + strconv.Itoa(cfg.Ratelimit.WindowLength) + " seconds\n"

	return ret
}
func LoadConfig(filePath string) (*Config, error) {
	AppConfig = &Config{}
	_, err := toml.DecodeFile(filePath, AppConfig)

	if err != nil {
		return nil, err
	}

	if err := AppConfig.Validate(); err != nil {
		return nil, err
	}

	AppConfig.JWT.SecretBytes = []byte(AppConfig.JWT.Secret)
	return AppConfig, err
}
