# Crypto Flash
A trading bot for cryptocurrency which aims to be fast.
Only support FTX exchange for now.
## 100 Days Backtesting (profit with 1M USD 1x leverge)
![Recent Backtest](/backtest-example.png)



## Requirements

### Stable version

* Golang versions: 1.15 up

## Install

Steps:

1. Install [GVM](https://github.com/moovweb/gvm)

```sh
$ zsh < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)
```

2. Install Go on Mac

```sh
# Install Go using binary
# run gvm install go1.15 -B if binary is available for this version
$ gvm install go1.15
$ gvm use go1.15
```
**if you are using MacOS Big Sur**
```sh
$ brew install go
$ gvm install go1.15
$ gvm use go1.15
$ brew uninstall go
```
[reference](https://github.com/moovweb/gvm/issues/360)

3. Download related pacakges

```sh

$ go mod download
```

### How to run the server?

```sh
$ go run main.go
```

### How to run the unit test of all test files?

```sh
$ go test -v ./...
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
    - `mode`: mode for running the program. Available values are `"trade"`, `"simulate"` and `"backtest"`.
        - Trade mode is for actual trading with the strategy, which means `key` and `secret` must be provided.
        - Notify mode is for simulation and notification. No actual trade will happen.
        - Backtest mode is for backtesting with historical price data.
    - `verbose`: Whether to send notification or not.
    - `key`: API key generated from FTX exchange
    - `secret`: API secret generated from FTX exchange
    - `subAccount`: Sub-account in FTX exchange
    - `telegramId`: User's telegram room id of the bot chat room to provide personal trading information. If provided, `owner` should match the telegram user name.
    - `strategy`: Strategy of the bot. Currently the available values are `"fr_arbitrage"` and `"res_trend"`.
- `line`: Line bot configuration.
- `telegram`: Telegram bot configuration. Just put the bot API token here.
- `sentry`: Sentry DSN configuration. Just put the DSN URL here.
