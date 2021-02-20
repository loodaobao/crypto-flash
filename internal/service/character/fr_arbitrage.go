/*
// Funding Rate Arbitrage is a signal provider utilizes funding rate on
// perpetual contract to earn profit.
// TODO: all
*/
package character

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	exchange "crypto-flash/internal/service/exchange"

	util "crypto-flash/internal/service/util"
)

type future struct {
	name            string
	spotPairName    string
	perpPairName    string
	quarterPairName string
	fundingRates    []float64
	// consecutive count of same sign funding rate
	consCount int
	// average APR for a period of time
	avgAPR float64
	// +: long on perp, -: short on perp
	size float64
	// hedge profit right now, set every hour
	currentHedgeProfit float64
	// profit this hour, set every hour
	hourlyFundingRateProfit float64
	// total profit of this pair from server start, accumulated
	totalProfit       float64
	perpEnterPrice    float64
	quarterEnterPrice float64
}
type FRArb struct {
	SignalProvider
	ftx *exchange.FTX
	// strategy config
	quarterContractName  string
	updatePeriod         int64
	reportPeriod         int64
	blacklistFutureNames []string
	leverage             float64
	longTime             int
	startAPRThreshold    float64
	endAPRThreshold      float64
	startSpreadRate      float64
	prevRateDays         int64
	minAmount            float64
	// data
	freeBalance  float64
	futures      map[string]*future
	startFutures []*future
	stopFutures  []*future
}

func NewFRArb(ftx *exchange.FTX, notifier *Notifier, owner string) *FRArb {
	return &FRArb{
		SignalProvider: SignalProvider{
			tag:             "FRArbProvider-" + owner,
			owner:           owner,
			startTime:       time.Now(),
			position:        nil,
			initBalance:     1000000,
			balance:         1000000,
			notifier:        notifier,
			signalChan:      nil,
			takeProfitCount: 0,
			stopLossCount:   0,
		},
		ftx: ftx,
		// config
		quarterContractName: "0326",
		// period of main loop in minute
		updatePeriod: 15,
		// period of report in hour
		reportPeriod:         1,
		blacklistFutureNames: []string{},
		// perp and quarter have 1/2 pairPortion and leverage
		leverage: 5,
		// consecutive hours of positive/negative funding rate
		longTime: 1 * 24,
		// start arbitrage if APR is more then this threshold
		startAPRThreshold: 5,
		// end arbitrage if APR is smaller then this threshold
		endAPRThreshold: -0.5,
		// spread should be smaller than startSpreadRate
		startSpreadRate: 0.02,
		// previous data we used to calculate avgAPR
		prevRateDays: 7,
		// minimum USD amount to start a pair (perp + quarter)
		minAmount: 10,
		// data
		futures:     make(map[string]*future),
		freeBalance: 1000000,
	}
}
func (fra *FRArb) fundingRateToAPR(fundingRate float64) float64 {
	return math.Abs(fundingRate) * 365 * 24 * fra.leverage / 2
}
func (fra *FRArb) Backtest(startTime, endTime int64) float64 {
	/*
		candles :=
			sh.ftx.GetHistoryCandles(sh.market, 300, startTime, endTime)
		util.Info(sh.tag, "start backtesting")
		for _, candle := range candles {
			sh.genSignal(candle.GetAvg(), candle.GetAvg())
		}
		roi := util.CalcROI(sh.initBalance, sh.balance)
		util.Info(sh.tag,
			fmt.Sprintf("balance: %.2f, total ROI: %.2f%%", sh.balance, roi * 100))
		return roi*/
	return 0
}
func (fra *FRArb) calculateFutureSpreadRate(futureName string) (float64, error) {
	prices, err := fra.ftx.GetFuture(futureName)
	if err != nil {
		return 0, err
	}
	return (prices.Ask - prices.Bid) / prices.Bid, nil
}
func (fra *FRArb) genSignal(future *future) {
	if future.quarterPairName == "" {
		return
	}
	fundingRates := future.fundingRates
	nextFundingRate := fundingRates[0]
	nextFundingAPR := fra.fundingRateToAPR(nextFundingRate)
	util.Info(fra.tag, future.name, fmt.Sprintf("next funding rate: %f", fundingRates[0]))
	util.Info(fra.tag, future.name, fmt.Sprintf("next equivalent APR: %.2f%%", nextFundingAPR*100))
	future.consCount = 1
	for i := 1; i < len(future.fundingRates); i++ {
		if fundingRates[i]*fundingRates[0] <= 0 {
			break
		}
		future.consCount++
	}
	totalRate := 0.0
	for _, rate := range fundingRates {
		totalRate += rate
	}
	toAnnual := float64(365*24) / float64(len(fundingRates))
	future.avgAPR = math.Abs(totalRate) * toAnnual * fra.leverage / 2
	util.Info(fra.tag, future.name, fmt.Sprintf("avgAPR: %.2f%%", future.avgAPR*100))
	shouldEnd := (future.size*nextFundingRate) > 0 && -nextFundingAPR <= fra.endAPRThreshold
	if shouldEnd {
		util.Info(fra.tag, "not profitable: "+future.name)
		fra.send("not profitable: " + future.name)
		fra.stopFutures = append(fra.stopFutures, future)
		return
	}
	perpSpreadRate, err := fra.calculateFutureSpreadRate(future.perpPairName)
	if err != nil {
		return
	}
	quarterSpreadRate, err := fra.calculateFutureSpreadRate(future.quarterPairName)
	if err != nil {
		return
	}
	shouldStart := nextFundingAPR >= fra.startAPRThreshold &&
		perpSpreadRate <= fra.startSpreadRate && quarterSpreadRate <= fra.startSpreadRate
	if shouldStart && future.size == 0 {
		util.Info(fra.tag, "profitable: "+future.name)
		fra.send("profitable: " + future.name + "\n" + fmt.Sprintf("avgAPR: %.2f%%", future.avgAPR*100))
		fra.startFutures = append(fra.startFutures, future)
	}
}

func (fra *FRArb) sortAPR() []string {
	type kv struct {
		k string
		v float64
	}
	var kvs []kv
	for name, future := range fra.futures {
		kvs = append(kvs, kv{name, future.avgAPR})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].v > kvs[j].v
	})
	var names []string
	for _, kv := range kvs {
		names = append(names, kv.k)
	}
	return names
}

func (fra *FRArb) sendTotalROIReport() {
	msg := "ROI Report\n"
	runTime := time.Now().Sub(fra.startTime)
	d := util.FromTimeDuration(runTime)
	msg += "runtime: " + d.String() + "\n\n"
	totalProfit := 0.0
	for _, future := range fra.futures {
		totalProfit += future.totalProfit
	}
	msg += fmt.Sprintf("total profit: %.2f\n", totalProfit)
	msg += fmt.Sprintf("current free balance %.2f\n", fra.freeBalance)
	currentBalance := fra.initBalance + totalProfit
	roi := util.CalcROI(fra.initBalance, currentBalance)
	msg += fmt.Sprintf("ROI: %.2f%%\n", roi*100)
	apr := util.CalcAnnualFromROI(roi, runTime.Seconds())
	msg += fmt.Sprintf("apr: %.2f%%", apr*100)
	util.Info(fra.tag, msg)
	fra.send(msg)
}
func (fra *FRArb) sendFutureStatusReport() {
	names := fra.sortAPR()
	for _, name := range names {
		future := fra.futures[name]
		if future.totalProfit != 0 {
			msg := fmt.Sprintf("%s\n", name)
			msg += fmt.Sprintf("size: %.2f\n", future.size)
			msg += fmt.Sprintf("avgAPR: %.2f%%\n", future.avgAPR*100)
			msg += fmt.Sprintf("consCount: %d\n", future.consCount)
			msg += fmt.Sprintf("next funding rate: %f (APR %.2f%%)\n",
				future.fundingRates[0], fra.fundingRateToAPR(future.fundingRates[0])*100)
			currentHedgeProfitROI := future.currentHedgeProfit / math.Abs(future.size)
			msg += fmt.Sprintf("current hedge profit: %.2f (%.2f%%)\n",
				future.currentHedgeProfit, currentHedgeProfitROI*100)
			msg += fmt.Sprintf("total profit: %.2f\n", future.totalProfit)
			msg += fmt.Sprintf("total profit + current hedge profit: %.2f\n",
				future.totalProfit+future.currentHedgeProfit)
			util.Info(fra.tag, msg)
			fra.send(msg)
		}
	}
}
func (fra *FRArb) startPair(future *future, size float64) {
	perpSide := "long"
	//quarterSide := "short"
	if future.fundingRates[0] > 0 {
		perpSide = "short"
		//quarterSide = "long"
		future.size = -size
	} else {
		future.size = size
	}
	/*
		// TODO: set stop loss
		fra.sendSignal(&util.Signal{
			Market: future.perpPairName,
			Side: perpSide,
			Reason: "Profitable",
			Ratio: ratio,
		})
		fra.sendSignal(&util.Signal{
			Market: future.quarterPairName,
			Side: quarterSide,
			Reason: "Profitable",
			Ratio: ratio,
		})
	*/
	perpPrices, err := fra.ftx.GetFuture(future.perpPairName)
	if err != nil {
		return
	}
	quarterPrices, err := fra.ftx.GetFuture(future.quarterPairName)
	if err != nil {
		return
	}
	if perpSide == "long" {
		future.perpEnterPrice = perpPrices.Ask
		future.quarterEnterPrice = quarterPrices.Bid
	} else {
		future.perpEnterPrice = perpPrices.Bid
		future.quarterEnterPrice = quarterPrices.Ask
	}
	future.totalProfit -= math.Abs(future.size) * fra.ftx.Fee * 2
	util.Info(fra.tag, fmt.Sprintf("start earning on %s, size %f", future.name, future.size))
	fra.send(fmt.Sprintf("start earning on %s, size %f", future.name, future.size))
}
func (fra *FRArb) calculateHedgeProfit(future *future) (float64, error) {
	perpPrices, err := fra.ftx.GetFuture(future.perpPairName)
	if err != nil {
		return 0, err
	}
	quarterPrices, err := fra.ftx.GetFuture(future.quarterPairName)
	if err != nil {
		return 0, err
	}
	var perpPrice, quarterPrice, perpProfit, quarterProfit float64
	if future.size > 0 {
		perpPrice = perpPrices.Bid
		quarterPrice = quarterPrices.Ask
	} else {
		perpPrice = perpPrices.Ask
		quarterPrice = quarterPrices.Bid
	}
	size := math.Abs(future.size)
	perpProfit = size*(perpPrice/future.perpEnterPrice) - size
	quarterProfit = size*(quarterPrice/future.quarterEnterPrice) - size
	if future.size > 0 {
		quarterProfit *= -1
	} else {
		perpProfit *= -1
	}
	return perpProfit + quarterProfit, nil
}
func (fra *FRArb) updateFutureProfit(future *future) {
	future.hourlyFundingRateProfit = future.size * future.fundingRates[1] * -1
	future.totalProfit += future.hourlyFundingRateProfit
	currentHedgeProfit, err := fra.calculateHedgeProfit(future)
	if err != nil {
		util.Error("cannot calculate hedge profit")
	}
	future.currentHedgeProfit = currentHedgeProfit
}
func (fra *FRArb) stopPair(future *future) {
	/*
		fra.sendSignal(&util.Signal{
			Market: future.perpPairName,
			Side: "close",
			Reason: "Not profitable",
		})
		fra.sendSignal(&util.Signal{
			Market: future.quarterPairName,
			Side: "close",
			Reason: "Not profitable",
		})
	*/
	future.totalProfit -= math.Abs(future.size) * fra.ftx.Fee * 2
	fra.updateFutureProfit(future)
	util.Info(fra.tag, fmt.Sprintf("stop earning on %s, size %f", future.name, future.size))
	util.Info(fra.tag, fmt.Sprintf("final hedge profit: %f", future.currentHedgeProfit))
	fra.send(fmt.Sprintf("stop earning on %s, size %f", future.name, future.size))
	fra.send(fmt.Sprintf("final hedge profit: %f", future.currentHedgeProfit))
	future.totalProfit += future.currentHedgeProfit
	pairPortion := math.Abs(future.size) / fra.leverage * 2
	fra.freeBalance += pairPortion
	future.size = 0
	future.currentHedgeProfit = 0
	future.hourlyFundingRateProfit = 0
}
func (fra *FRArb) createFutures() {
	marketPairs, _ := fra.ftx.GetMarketPairs()
	quarterPairs := make(map[string]bool)
	spotPairs := make(map[string]bool)
	for _, name := range marketPairs.Quarters {
		existIndex := strings.Index(name, fra.quarterContractName)
		if existIndex != -1 {
			quarterPairs[name] = true
		}
	}
	for _, name := range marketPairs.Spots {
		usdExistIndex := strings.Index(name, "/USD")
		usdtExistIndex := strings.Index(name, "/USDT")
		if usdExistIndex != -1 && usdtExistIndex == -1 {
			spotPairs[name] = true
		}
	}
	// get previous funding rate
	now := time.Now().Unix()
	end := now - now%(60*60)
	start := end - fra.prevRateDays*24*60*60
	for _, perpPairName := range marketPairs.Perps {
		spotName := strings.Split(perpPairName, "-")[0]
		quarterPairName := spotName + "-" + fra.quarterContractName
		spotPairName := spotName + "/USD"
		_, isSpotPairExist := spotPairs[spotPairName]
		_, isQuarterPairExist := quarterPairs[quarterPairName]
		if isSpotPairExist || isQuarterPairExist {
			f := &future{
				name:         spotName,
				perpPairName: perpPairName,
			}
			if isSpotPairExist {
				f.spotPairName = spotPairName
			}
			if isQuarterPairExist {
				f.quarterPairName = quarterPairName
			}
			f.fundingRates = fra.ftx.GetFundingRates(start, end, perpPairName)
			fra.futures[spotName] = f
		} else {
			util.Info(fra.tag, fmt.Sprintf("%s cannot be used", spotName))
		}
	}
}
func (fra *FRArb) Start() {
	value, exist := os.LookupEnv("ENV")
	isTestEnv := exist && value == "test"
	fra.createFutures()
	for {
		now := time.Now().Unix()
		// TODO: check existing position every updatePeriod
		// one hour and 15 second just passed, get next funding rate
		getFundingRateOffset := 60*60 - fra.updatePeriod*2
		if now%(60*60) == getFundingRateOffset || isTestEnv {
			for _, future := range fra.futures {
				resp := fra.ftx.GetFutureStats(future.perpPairName)
				nextFundingRate := resp.NextFundingRate
				end := int(24*fra.prevRateDays - 1)
				if end > len(future.fundingRates) {
					end = len(future.fundingRates)
				}
				future.fundingRates = append([]float64{nextFundingRate}, future.fundingRates[:end]...)
				fra.genSignal(future)
			}
			for _, future := range fra.stopFutures {
				fra.stopPair(future)
			}
			util.Info(fra.tag,
				fmt.Sprintf("free balance: %f, start pair count: %d", fra.freeBalance, len(fra.startFutures)))
			startFutureCount := float64(len(fra.startFutures))
			if startFutureCount > 0 && fra.freeBalance >= fra.minAmount*startFutureCount {
				pairPortion := fra.freeBalance / startFutureCount
				size := pairPortion / 2 * fra.leverage
				for _, future := range fra.startFutures {
					fra.startPair(future, size)
				}
				fra.freeBalance = 0
			}
			// update profit and current hedge profit
			isInStartFutures := func(future *future) bool {
				for _, startFuture := range fra.startFutures {
					if future == startFuture {
						return true
					}
				}
				return false
			}
			for name, future := range fra.futures {
				// do not update profit if future just starts
				if isInStartFutures(future) {
					continue
				}
				if future.size != 0 {
					fra.updateFutureProfit(future)
					msg := fmt.Sprintf("earned %.2f USD on %s", future.hourlyFundingRateProfit, name)
					util.Info(fra.tag, msg)
					fra.send(msg)
				}
			}
			fra.sendFutureStatusReport()
			fra.startFutures = fra.startFutures[:0]
			fra.stopFutures = fra.stopFutures[:0]
			// log rank
			names := fra.sortAPR()
			util.Info(fra.tag, "avgAPR Rank:")
			for _, name := range names {
				future := fra.futures[name]
				msg := fmt.Sprintf(
					"future: %s, avgAPR: %.2f%%, nextAPR: %.2f%%, consCount: %d",
					name, future.avgAPR*100, fra.fundingRateToAPR(future.fundingRates[0])*100, future.consCount)
				util.Info(fra.tag, msg)
			}
		}
		// generate roi report
		if now%(fra.reportPeriod*60*60) == getFundingRateOffset {
			fra.sendTotalROIReport()
		}
		timeToNextCycle := fra.updatePeriod - time.Now().Unix()%fra.updatePeriod
		sleepDuration := util.Duration{Second: timeToNextCycle}
		time.Sleep(sleepDuration.GetTimeDuration())
	}
}
