package config

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/BurntSushi/toml"
)

type (
	Config struct {
		Server    server
		Ratelimit ratelimit
		Database  database
	}
	server struct {
		HostName string
		Port     int
		// PublicURL is the externally-reachable base URL clients use to
		// contact this server (e.g. "https://tillit.example.com"). It's
		// embedded in signed auth tokens so a token signed for one
		// server can't be replayed against another. Required when
		// authenticated endpoints are in use.
		PublicURL string
	}

	ratelimit struct {
		RequestLimit int

		// Window length in seconds
		WindowLength int
	}

	database struct {
		Type string
		DSN  string
	}
)

func (cfg *Config) Validate() error {
	if err := cfg.Server.Validate(); err != nil {
		return err
	}

	if err := cfg.Ratelimit.Validate(); err != nil {
		return err
	}

	if err := cfg.Database.Validate(); err != nil {
		return err
	}

	return nil
}

func (s *server) Validate() error {
	if len(s.HostName) == 0 {
		return errors.New("server.HostName must not be empty")
	}
	if s.Port <= 0 || s.Port > 65535 {
		return errors.New("server.Port needs to be between 1 and 65535")
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

func (d *database) Validate() error {

	switch d.Type {
	case "sqlite":
	case "":
		return errors.New("Database type missing")
	default:
		return fmt.Errorf("unknown DB type: %q", d.Type)
	}

	if len(d.DSN) == 0 {
		return errors.New("Missing database DSN")
	}

	return nil
}

var AppConfig *Config

func (cfg *Config) String() string {

	ret := "Config\n"
	ret += "Server:\n"
	ret += "\tHostName: " + cfg.Server.HostName + "\n"
	ret += "\tPort: " + strconv.Itoa(cfg.Server.Port) + "\n"
	ret += "Ratelimit:\n"
	ret += "\tRequestLimit: " + strconv.Itoa(cfg.Ratelimit.RequestLimit) + "\n"
	ret += "\tWindowLength: " + strconv.Itoa(cfg.Ratelimit.WindowLength) + " seconds\n"
	ret += "Database:\n"
	ret += "\tType: " + cfg.Database.Type + "\n"
	ret += "\tDSN: " + cfg.Database.DSN + "\n"

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

	return AppConfig, err
}
