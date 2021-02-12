# Crypto Flash
A trading bot for cryptocurrency which aims to be fast.
Only support FTX exchange for now.
## 100 Days Backtesting (profit with 1M USD 1x leverge)
![Recent Backtest](/backtest-example.png)



## Requirements

### Stable version

* Golang versions: 1.14 up

## Install

Steps:

1. Install [GVM](https://github.com/moovweb/gvm)

```sh
$ zsh < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)
```

2. Install Go on Mac

```sh
# Install Go using binary
$ gvm install go1.15 -B
$ gvm use go1.15
```

3. Download related pacakges

```sh

$ go mod download
```

### How to run the server?

```sh
$ go run main.go
```

### How to hot reload server in development?

* Reference:
    - https://github.com/cosmtrek/air

#### Install Air

```sh
$ # binary will be $(go env GOPATH)/bin/air
curl -sSfL https://raw.githubusercontent.com/cosmtrek/air/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

```sh
# Run air with your config. If file name is `.air.toml`, just run `air`.
$ air

# prints all logs
$ air -d
```

## Configuration
Rename `config.json.example` file to `config.json` and fill in the following configurations:
- `bots`: An array of bot instance
    - `owner`: Name of the bot owner
    - `key`: API key generated from FTX exchange
    - `secret`: API secret generated from FTX exchange
    - `subAccount`: Sub-account in FTX exchange
    - `telegramId`: User's telegram room id of the bot chat room to provide personal trading information. If provided, `owner` should match the telegram user name.
    - `strategy`: Strategy of the bot. Currently the available values are `"fr_arbitrage"` and `"res_trend"`.
- `mode`: mode for running the program. Available values are `"trade"`, `"notify"` and `"backtest"`.
    - Trade mode is for actual trading with the strategy, which means `key` and `secret` must be provided for every bot.
    - Notify mode is for simulation and notification. No actual trade will happen.
    - Backtest mode is for backtesting with historical price data.
- `notify`: Control whether to send notification or not.
- `line`: Line bot configuration.
- `telegram`: Telegram bot configuration. Just put the bot API token here.
