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
$ gvm install go1.14 -B
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
