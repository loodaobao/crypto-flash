package config

import (
	"crypto-flash/pkg/util"
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
}

type lineConfig struct {
	ChannelSecret      string
	ChannelAccessToken string
}

type config struct {
	Mode     string
	Notify   bool
	Bots     []bot
	Line     lineConfig
	Telegram string
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
