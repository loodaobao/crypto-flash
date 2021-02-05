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
	"sync"
	"time"

	character "github.com/CheshireCatNick/crypto-flash/pkg/character"
	exchange "github.com/CheshireCatNick/crypto-flash/pkg/exchange"
	util "github.com/CheshireCatNick/crypto-flash/pkg/util"

	config "crypto-flash/config"
)

const (
	version = "3.4.2-beta"
	update  = "1. Add funding rate arbitrage provider for simulation test.\n" +
		"2. Funding pool logic test.\n" +
		"3. Funding rate arbitrage profit calculation includes trading fee and hedge profit (or cost)."
	tag = "Crypto Flash"
)

func main() {
	var wg sync.WaitGroup
	fmt.Printf("Crypto Flash v%s initialized. Update: \n%s\n", version, update)

	config := config.Load("config.json", tag)

	var n *character.Notifier
	if config.Notify && config.Mode != "backtest" {
		n = character.NewNotifier(config.Line.ChannelSecret,
			config.Line.ChannelSecret, config.Telegram)
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
	if config.Mode == "trade" {
		for _, bot := range config.Bots {
			if bot.Key == "" || bot.Secret == "" {
				continue
			}
			ftx := exchange.NewFTX(bot.Key, bot.Secret, bot.SubAccount)
			trader := character.NewTrader(bot.Owner, ftx, n)
			signalChan := make(chan *util.Signal)
			rs.SubSignal(signalChan)
			wg.Add(1)
			go trader.Start(signalChan)
		}
		wg.Add(2)
		go rs.Start()
		go fra.Start()
	} else if config.Mode == "notify" {
		wg.Add(2)
		go rs.Start()
		go fra.Start()
	} else if config.Mode == "backtest" {
		//endTime, _ := time.Parse(time.RFC3339, "2019-12-01T05:00:00+00:00")
		endTime := time.Now()
		d := util.Duration{Day: -60}
		startTime := endTime.Add(d.GetTimeDuration())
		roi := rs.Backtest(startTime.Unix(), endTime.Unix())
		annual := util.CalcAnnualFromROI(roi, -d.GetTimeDuration().Seconds())
		fmt.Printf("Annual: %.2f%%", annual*100)
	}
	wg.Wait()
}
