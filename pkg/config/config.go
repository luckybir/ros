package config

import (
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Config struct {
	AsyncHost string `yaml:"asyncHost"`
	AsyncSapRoute []string `yaml:"asyncSapRoute,flow"`
}

var ServerConfig Config

func InitConfig()  {
	configFile, err := ioutil.ReadFile("./conf/config.yaml")
	if err != nil {
		zap.S().Fatalf("read config err:%s", err.Error())
	}

	err = yaml.Unmarshal(configFile, &ServerConfig)
	if err != nil {
		zap.S().Fatalf("read config err:%s", err.Error())
	}

	zap.S().Warnf("async host:%s", ServerConfig.AsyncHost)

	for _,asyncPath := range ServerConfig.AsyncSapRoute{
		zap.S().Warnf("async path:%s", asyncPath)
	}


}
