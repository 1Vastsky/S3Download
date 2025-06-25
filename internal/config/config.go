package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Routes struct {
	Start  string `yaml:"start"`
	Status string `yaml:"status"`
	List   string `yaml:"list"`
}

type Config struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	Bucket          string `yaml:"bucket"`
	Dest            string `yaml:"dest"`
	Concurrency     int    `yaml:"concurrency"`
	PartSizeMiB     int    `yaml:"part_size_mib"`
	MaxRetries      int    `yaml:"max_retries"`
	Routes          Routes `yaml:"routes"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var c Config
	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}

	if c.Routes.List == "" {
		c.Routes.List = "/objects"
	}

	// 环境变量覆盖
	if ak := os.Getenv("OSS_AK_ID"); ak != "" {
		c.AccessKeyID = ak
	}
	if sk := os.Getenv("OSS_AK_SECRET"); sk != "" {
		c.AccessKeySecret = sk
	}
	if s := os.Getenv("DOWNLOAD_ROUTE_START"); s != "" {
		c.Routes.Start = s
	}
	if s := os.Getenv("DOWNLOAD_ROUTE_STATUS"); s != "" {
		c.Routes.Status = s
	}
	if !strings.Contains(c.Routes.Status, "{id}") {
		c.Routes.Status += "/{id}"
	}

	// 合理默认
	if c.Concurrency <= 0 {
		c.Concurrency = 16
	}
	if c.PartSizeMiB < 8 {
		c.PartSizeMiB = 8
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.Dest == "" {
		c.Dest = "./data"
	}
	return &c, nil
}
