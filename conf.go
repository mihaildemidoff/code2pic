package main

import (
	"github.com/micro/go-config"
	"github.com/micro/go-config/source/file"
)

type TelegramSettings struct {
	Secret  string `json:"secret"`
	Timeout int    `json:"timeout"`
	Debug   bool   `json:"debug"`
}

type RedisSettings struct {
	Url                  string `json:"url"`
	Password             string `json:"password"`
	DefaultRecordTimeout int64  `json:"defaultRecordTimeout"`
}

type PhantomjsSettings struct {
	Url          string `json:"url"`
	WindowWidth  int    `json:"windowWidth"`
	WindowHeight int    `json:"windowHeight"`
}

type HttpServerSettings struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

type Settings struct {
	TelegramSettings                `json:"telegram"`
	RedisSettings                   `json:"redis"`
	PhantomjsSettings               `json:"phantomjs"`
	HttpServerSettings              `json:"httpServer"`
	TemplateAddress          string `json:"templateAddress"`
	NumberOfGeneratorWorkers int    `json:"numberOfGeneratorWorkers"`
}

func loadConfig() (*Settings, error) {
	conf := config.NewConfig()
	err := conf.Load(file.NewSource(file.WithPath("conf.json")))
	if err != nil {
		return nil, err
	}
	var settings = new(Settings)
	if err = conf.Get().Scan(settings); err != nil {
		return nil, err
	}
	return settings, nil
}
