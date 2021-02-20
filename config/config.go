package config

import (
	"crypto-flash/internal/service/util"
	"encoding/json"
	"io/ioutil"
)

type bot struct {
	Owner      string
	Key        string
	Secret     string
	SubAccount string
	TelegramID int64
	Strategy   string
	Mode       string
	Verbose    bool
}

type lineConfig struct {
	ChannelSecret      string
	ChannelAccessToken string
}

type config struct {
	Bots     []bot
	Line     lineConfig
	Telegram string
	Sentry   string
}

func Load(fileName, tag string) config {
	var c config
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		util.Error(tag, err.Error())
	}
	json.Unmarshal(bytes, &c)
	return c
}
