/*
// TODO:
// 1. tests and DB
// 2. consider having exchange interface, signal provider interface
// 3. auto-backtesting and parameter optimization with report to notifier
// 4. funding rate arbitrage
*/
package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"crypto-flash/internal/apm"
	character "crypto-flash/internal/service/character"
	exchange "crypto-flash/internal/service/exchange"

	util "crypto-flash/internal/service/util"

	config "crypto-flash/config"

	"github.com/joho/godotenv"
)

const (
	version = "3.4.2-beta"
	update  = "1. Add funding rate arbitrage provider for simulation test.\n" +
		"2. Funding pool logic test.\n" +
		"3. Funding rate arbitrage profit calculation includes trading fee and hedge profit (or cost)."
	tag = "Crypto Flash"
)

func init() {
	_ = godotenv.Load()
}

func main() {
	config := config.Load("config.json", tag)
	value, exist := os.LookupEnv("ENV")
	isTestEnv := exist && value == "test"
	// Init sentry service
	apm.InitSentryService(config.Sentry)
	var wg sync.WaitGroup
	fmt.Printf("Crypto Flash v%s initialized. Update: \n%s\n", version, update)
	// create notifier
	var n *character.Notifier
	if config.Telegram != "" && !isTestEnv {
		n = character.NewNotifier(config.Line.ChannelSecret, config.Line.ChannelSecret, config.Telegram)
		wg.Add(1)
		go n.Listen()
	} else {
		n = nil
	}
	// create bots
	// TODO: some resource like orderbook should be shared between FTX instance
	for _, bot := range config.Bots {
		if bot.Mode == "backtest" {
			ftx := exchange.NewFTX("", "", "")
			rs := character.NewResTrend(ftx, nil)
			//endTime, _ := time.Parse(time.RFC3339, "2019-12-01T05:00:00+00:00")
			endTime := time.Now()
			d := util.Duration{Day: -60}
			startTime := endTime.Add(d.GetTimeDuration())
			roi := rs.Backtest(startTime.Unix(), endTime.Unix())
			annual := util.CalcAnnualFromROI(roi, -d.GetTimeDuration().Seconds())
			fmt.Printf("Annual: %.2f%%", annual*100)
		} else if bot.Mode == "simulate" || bot.Mode == "trade" {
			if bot.Mode == "trade" && (bot.Key == "" || bot.Secret == "") {
				continue
			}
			if bot.Verbose && n != nil {
				n.AddUser(bot.Owner, bot.TelegramID)
				n.Send(tag, bot.Owner, fmt.Sprintf("Crypto Flash v%s initialized. Update: \n%s", version, update))
			}
			ftx := exchange.NewFTX("", "", "")
			if bot.Mode == "trade" {
				ftx = exchange.NewFTX(bot.Key, bot.Secret, bot.SubAccount)
			}
			// TODO: different strategy run different bot
			fra := character.NewFRArb(ftx, n, bot.Owner)
			wg.Add(1)
			go fra.Start()
		}
	}
	wg.Wait()
	/*
		var n *character.Notifier
		if config.Notify && config.Mode != "backtest" {
			n = character.NewNotifier(config.Line.ChannelSecret, config.Line.ChannelSecret, config.Telegram)
			for _, bot := range config.Bots {
				n.AddUser(bot.Owner, bot.TelegramID)
			}
			wg.Add(1)
			go n.Listen()
			n.Broadcast(tag,
				fmt.Sprintf("Crypto Flash v%s initialized. Update: \n%s",
					version, update))
		} else {
			n = nil
		}
		ftx := exchange.NewFTX("", "", "")
		rs := character.NewResTrend(ftx, n)
		fra := character.NewFRArb(ftx, n)
		startSignalProvider := func() {
			//go rs.Start()
			//wg.Add(1)
			go fra.Start()
			wg.Add(1)
		}
		if config.Mode == "trade" {
			// TODO: each trader should have its own signal provider
			// or we should merge trader with exchange
			for _, bot := range config.Bots {
				if bot.Key == "" || bot.Secret == "" {
					continue
				}
				ftx := exchange.NewFTX(bot.Key, bot.Secret, bot.SubAccount)
				trader := character.NewTrader(bot.Owner, ftx, n)
				signalChan := make(chan *util.Signal)
				if bot.Strategy == "fr_arbitrage" {
					fra.SubSignal(signalChan)
				} else if bot.Strategy == "res_trend" {
					rs.SubSignal(signalChan)
				}
				wg.Add(1)
				go trader.Start(signalChan)
			}
			startSignalProvider()
		} else if config.Mode == "notify" {
			startSignalProvider()
		} else if config.Mode == "backtest" {
			//endTime, _ := time.Parse(time.RFC3339, "2019-12-01T05:00:00+00:00")
			endTime := time.Now()
			d := util.Duration{Day: -60}
			startTime := endTime.Add(d.GetTimeDuration())
			roi := rs.Backtest(startTime.Unix(), endTime.Unix())
			annual := util.CalcAnnualFromROI(roi, -d.GetTimeDuration().Seconds())
			fmt.Printf("Annual: %.2f%%", annual*100)
		}
		wg.Wait()*/
}
