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

2. Install Go

```sh
$ gvm install go1.4
$ gvm use go1.14
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

```sh
# Run air with your config. If file name is `.air.toml`, just run `air`.
$ air

# prints all logs
$ air -d
```