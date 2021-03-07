/*
// Funding Rate Arbitrage is a signal provider utilizes funding rate on
// perpetual contract to earn profit.
*/
package character

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	exchange "crypto-flash/internal/service/exchange"

	util "crypto-flash/internal/service/util"
)

type future struct {
	name           string
	spotPair       string
	perpPair       string
	quarterPair    string
	hedgePair      string
	isCollaterable bool
	fundingRates   []float64
	// next funding rate in the prior half hour is the previous funding rate (for more accuracy)
	// next funding rate in the later half hour is predicted funding rate
	nextFundingRate float64
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
	totalProfit     float64
	perpEnterPrice  float64
	hedgeEnterPrice float64
}
type FRArb struct {
	SignalProvider
	ftx        *exchange.FTX
	orderbooks map[string]*util.Orderbook
	// strategy config
	quarterContractName       string
	blacklistFutureNames      []string
	leverage                  float64
	longTime                  int
	startAPRThreshold         float64
	stopAPRThreshold          float64
	startBuySellSpreadRate    float64
	startFutureSpotSpreadRate float64
	stopFutureSpotSpreadRate  float64
	increaseSizeThreshold     float64
	prevRateDays              int64
	minAmount                 float64
	freeBalanceAllocateRate   float64
	// data
	freeBalance            float64
	futures                map[string]*future
	startFuturesInThisHour map[string]bool
}

func NewFRArb(ftx *exchange.FTX, notifier *Notifier, owner string, orderbooks map[string]*util.Orderbook) *FRArb {
	return &FRArb{
		SignalProvider: SignalProvider{
			tag:             "FRArb-" + owner,
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
		ftx:        ftx,
		orderbooks: orderbooks,
		// config
		quarterContractName:  "0326",
		blacklistFutureNames: []string{},
		// perp and quarter have 1/2 pairPortion and leverage
		leverage: 5,
		// consecutive hours of positive/negative funding rate
		longTime: 1 * 24,
		// start arbitrage if APR is more then this threshold
		startAPRThreshold: 2,
		// stop arbitrage if APR is smaller then this threshold
		stopAPRThreshold: 1,
		// buy sell spread should be smaller than startSpreadRate
		startBuySellSpreadRate: 0.01,
		// future spot spread should be larger to start position
		// start - stop should > fee (0.0007 * 4)
		startFutureSpotSpreadRate: 0.003,
		// future spot spread should be smaller to stop position
		stopFutureSpotSpreadRate: 0,
		// increase size if inner spread is getting larger
		increaseSizeThreshold: 0.0005,
		// previous data we used to calculate avgAPR
		prevRateDays: 7,
		// minimum USD amount to start a pair (perp + quarter)
		minAmount: 10,
		// each time we allocate this rate of balance
		freeBalanceAllocateRate: 0.2,
		// data
		futures:     make(map[string]*future),
		freeBalance: 10000,
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
func (fra *FRArb) getOrderbook(marketPair string) *util.Orderbook {
	// restful
	//return fra.ftx.GetOrderbook(marketPair, 1)
	// ws
	return fra.orderbooks[marketPair]
}
func (fra *FRArb) calculateSpreadRate(marketPair string) (float64, error) {
	ob := fra.getOrderbook(marketPair)
	sellPrice, err := ob.GetMarketSellPrice()
	if err != nil {
		return -1, err
	}
	buyPrice, err := ob.GetMarketBuyPrice()
	if err != nil {
		return -1, err
	}
	return (sellPrice - buyPrice) / buyPrice, nil
}
func (fra *FRArb) calculateInnerSpreadRate(highOrderbook, lowOrderbook *util.Orderbook) (float64, error) {
	// high pair is the one we want to short
	highPrice, err := highOrderbook.GetMarketSellPrice()
	if err != nil {
		return -1, err
	}
	lowPrice, err := lowOrderbook.GetMarketBuyPrice()
	if err != nil {
		return -1, err
	}
	return (highPrice - lowPrice) / lowPrice, nil
}
func (fra *FRArb) calculateOuterSpreadRate(highOrderbook, lowOrderbook *util.Orderbook) (float64, error) {
	// high pair is the one we want to buy back
	highPrice, err := highOrderbook.GetMarketBuyPrice()
	if err != nil {
		return -1, err
	}
	lowPrice, err := lowOrderbook.GetMarketSellPrice()
	if err != nil {
		return -1, err
	}
	return (highPrice - lowPrice) / lowPrice, nil
}
func (fra *FRArb) calculateEnterSpreadRate(future *future) float64 {
	if future.size == 0 {
		return 0.0
	}
	highPrice := math.Max(future.perpEnterPrice, future.hedgeEnterPrice)
	lowPrice := math.Min(future.perpEnterPrice, future.hedgeEnterPrice)
	return (highPrice - lowPrice) / lowPrice
}
func (fra *FRArb) genSignal(future *future) (bool, bool) {
	highPair := future.perpPair
	lowPair := future.hedgePair
	nextFundingRate := future.nextFundingRate
	if nextFundingRate < 0 {
		highPair, lowPair = lowPair, highPair
	}
	highOrderbook := fra.getOrderbook(highPair)
	lowOrderbook := fra.getOrderbook(lowPair)
	outerSpreadRate, err := fra.calculateOuterSpreadRate(highOrderbook, lowOrderbook)
	shouldStop, shouldStart := false, false
	nextFundingAPR := fra.fundingRateToAPR(nextFundingRate)
	if err == nil {
		//util.Info(fra.tag, fmt.Sprintf("%s outer spread rate: %.4f", future.name, outerSpreadRate))
		curNextFundingAPR := 0.0
		if future.size*nextFundingRate > 0 {
			curNextFundingAPR = -nextFundingAPR
		} else if future.size*nextFundingRate < 0 {
			curNextFundingAPR = nextFundingAPR
		}
		shouldStop = future.size != 0 && curNextFundingAPR < fra.stopAPRThreshold &&
			outerSpreadRate <= fra.stopFutureSpotSpreadRate
		if shouldStop {
			fra.stopPair(future)
			stopReason := "funding rate not profitable" // please update when shouldStop logic change
			if outerSpreadRate <= fra.stopFutureSpotSpreadRate {
				stopReason = "outer spread smaller than threshold"
			}
			msg := fmt.Sprintf("not profitable: %s\nstop reason: %s\nnextAPR: %.2f%%",
				future.name, stopReason, nextFundingAPR*100)
			util.Info(fra.tag, msg)
			fra.send(msg)
			return shouldStop, shouldStart
		}
	}
	innerSpreadRate, err := fra.calculateInnerSpreadRate(highOrderbook, lowOrderbook)
	if err == nil {
		//util.Info(fra.tag, fmt.Sprintf("%s inner spread rate %.4f\n", future.name, innerSpreadRate))
		canPerfectLeverage := nextFundingRate < 0 || future.isCollaterable
		// TODO: shouldIncreaseSize
		shouldStart = future.size == 0 && nextFundingAPR >= fra.startAPRThreshold &&
			innerSpreadRate >= fra.startFutureSpotSpreadRate && canPerfectLeverage
		enterSpreadRate := fra.calculateEnterSpreadRate(future)
		shouldIncrease := future.size != 0 && nextFundingAPR >= fra.startAPRThreshold &&
			innerSpreadRate-enterSpreadRate >= fra.increaseSizeThreshold && canPerfectLeverage
		if shouldStart || shouldIncrease {
			allocatedBalance := fra.freeBalance * fra.freeBalanceAllocateRate
			size := allocatedBalance / 2 * fra.leverage
			fra.increasePairSize(future, size)
			fra.freeBalance -= allocatedBalance
			msg := fmt.Sprintf("profitable: %s\navgAPR: %.2f%%\nnextAPR: %.2f%%\nincrease: %t",
				future.name, future.avgAPR*100, nextFundingAPR*100, shouldIncrease)
			util.Info(fra.tag, msg)
			fra.send(msg)
		}
	}
	return shouldStop, shouldStart
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
	totalHedgeProfit := 0.0
	for _, future := range fra.futures {
		totalProfit += future.totalProfit
		totalHedgeProfit += future.currentHedgeProfit
	}
	msg += fmt.Sprintf("total profit: %.2f\n", totalProfit)
	msg += fmt.Sprintf("total profit w/ hedge: %.2f\n", totalProfit+totalHedgeProfit)
	msg += fmt.Sprintf("current free balance %.2f\n", fra.freeBalance)
	currentBalance := fra.initBalance + totalProfit + totalHedgeProfit
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
		if future.size != 0 {
			msg := fmt.Sprintf("%s\n", name)
			msg += fmt.Sprintf("size: %.2f\n", future.size)
			msg += fmt.Sprintf("avgAPR: %.2f%%\n", future.avgAPR*100)
			msg += fmt.Sprintf("consCount: %d\n", future.consCount)
			msg += fmt.Sprintf("next funding rate: %f (APR %.2f%%)\n",
				future.nextFundingRate, fra.fundingRateToAPR(future.nextFundingRate)*100)
			currentHedgeProfitROI := future.currentHedgeProfit / math.Abs(future.size)
			msg += fmt.Sprintf("hedgePair: %s\n", future.hedgePair)
			canPerfectLeverage := future.size >= 0 || future.isCollaterable
			msg += fmt.Sprintf("isCollaterable: %t, canPerfectLeverage: %t\n", future.isCollaterable, canPerfectLeverage)
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
	if future.nextFundingRate > 0 {
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
	perpOrderbook := fra.getOrderbook(future.perpPair)
	hedgeOrderbook := fra.getOrderbook(future.hedgePair)
	if perpSide == "long" {
		future.perpEnterPrice, _ = perpOrderbook.GetMarketBuyPrice()
		future.hedgeEnterPrice, _ = hedgeOrderbook.GetMarketSellPrice()
	} else {
		future.perpEnterPrice, _ = perpOrderbook.GetMarketSellPrice()
		future.hedgeEnterPrice, _ = hedgeOrderbook.GetMarketBuyPrice()
	}
	future.totalProfit -= math.Abs(future.size) * fra.ftx.Fee * 2
	util.Info(fra.tag, fmt.Sprintf("start earning on %s, size %f", future.name, future.size))
	fra.send(fmt.Sprintf("start earning on %s, size %f", future.name, future.size))
}
func (fra *FRArb) increasePairSize(future *future, size float64) {
	originalSize := math.Abs(future.size)
	perpSide := "long"
	//quarterSide := "short"
	if future.nextFundingRate > 0 {
		perpSide = "short"
		//quarterSide = "long"
		future.size += -size
	} else {
		future.size += size
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
	perpOrderbook := fra.getOrderbook(future.perpPair)
	hedgeOrderbook := fra.getOrderbook(future.hedgePair)
	var curPerpPrice, curHedgePrice float64
	if perpSide == "long" {
		curPerpPrice, _ = perpOrderbook.GetMarketBuyPrice()
		curHedgePrice, _ = hedgeOrderbook.GetMarketSellPrice()
	} else {
		curPerpPrice, _ = perpOrderbook.GetMarketSellPrice()
		curHedgePrice, _ = hedgeOrderbook.GetMarketBuyPrice()
	}
	util.Info(fra.tag, fmt.Sprintf("increase size %f on %s", size, future.name))
	fra.send(fmt.Sprintf("increase size %f on %s", size, future.name))
	msg := fmt.Sprintf("opPrice %f, ohPrice %f, cpPrice %f, chPrice %f\n",
		future.perpEnterPrice, future.hedgeEnterPrice, curPerpPrice, curHedgePrice)
	msg += fmt.Sprintf("original size: %f, original enter spread %f\n",
		originalSize, fra.calculateEnterSpreadRate(future))
	future.perpEnterPrice = (originalSize*future.perpEnterPrice + size*curPerpPrice) / math.Abs(future.size)
	future.hedgeEnterPrice = (originalSize*future.hedgeEnterPrice + size*curHedgePrice) / math.Abs(future.size)
	msg += fmt.Sprintf("npPrice %f, nhPrice %f\n", future.perpEnterPrice, future.hedgeEnterPrice)
	msg += fmt.Sprintf("new size: %f, new enter spread %f", future.size, fra.calculateEnterSpreadRate(future))
	fra.send(msg)
	future.totalProfit -= math.Abs(size) * fra.ftx.Fee * 2
}
func (fra *FRArb) calculateHedgeProfit(future *future) (float64, error) {
	perpOrderbook := fra.getOrderbook(future.perpPair)
	hedgeOrderbook := fra.getOrderbook(future.hedgePair)
	var perpPrice, hedgePrice, perpProfit, hedgePairProfit float64
	if future.size > 0 {
		perpPrice, _ = perpOrderbook.GetMarketSellPrice()
		hedgePrice, _ = hedgeOrderbook.GetMarketBuyPrice()
	} else {
		perpPrice, _ = perpOrderbook.GetMarketBuyPrice()
		hedgePrice, _ = hedgeOrderbook.GetMarketSellPrice()
	}
	size := math.Abs(future.size)
	perpProfit = size*(perpPrice/future.perpEnterPrice) - size
	hedgePairProfit = size*(hedgePrice/future.hedgeEnterPrice) - size
	if future.size > 0 {
		hedgePairProfit *= -1
	} else {
		perpProfit *= -1
	}
	return perpProfit + hedgePairProfit, nil
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
		quarterPair := spotName + "-" + fra.quarterContractName
		spotPair := spotName + "/USD"
		_, isSpotPairExist := spotPairs[spotPair]
		_, isQuarterPairExist := quarterPairs[quarterPair]
		if isSpotPairExist {
			_, isCollaterable := fra.ftx.CollaterableSpots[spotName]
			f := &future{
				name:           spotName,
				perpPair:       perpPairName,
				isCollaterable: isCollaterable,
			}
			if isSpotPairExist {
				f.spotPair = spotPair
			}
			if isQuarterPairExist {
				f.quarterPair = quarterPair
			}
			if f.spotPair != "" {
				f.hedgePair = spotPair
			} else {
				f.hedgePair = quarterPair
			}
			f.fundingRates = fra.ftx.GetFundingRates(start, end, perpPairName)
			fra.futures[spotName] = f
		} else {
			util.Info(fra.tag, fmt.Sprintf("%s cannot be used", spotName))
		}
	}
}
func (fra *FRArb) GetRequiredPairs() []string {
	fra.createFutures()
	var pairs []string
	for _, future := range fra.futures {
		pairs = append(pairs, future.perpPair)
		pairs = append(pairs, future.spotPair)
	}
	return pairs
}
func (fra *FRArb) updateFundingRateProfit() {
	// runs at 30 sec before each hour
	beforeHourPeriod := 30
	for {
		now := time.Now().Unix() % 3600
		targetTime := int64(3600 - beforeHourPeriod)
		timeToNextCycle := targetTime - now
		if timeToNextCycle < 0 {
			timeToNextCycle += 3600
		}
		sleepDuration := util.Duration{Second: timeToNextCycle}
		time.Sleep(sleepDuration.GetTimeDuration())
		for _, future := range fra.futures {
			resp := fra.ftx.GetFutureStats(future.perpPair)
			future.nextFundingRate = resp.NextFundingRate
			end := int(24*fra.prevRateDays - 1)
			if end > len(future.fundingRates) {
				end = len(future.fundingRates)
			}
			future.fundingRates = append([]float64{future.nextFundingRate}, future.fundingRates[:end]...)
			fundingRates := future.fundingRates
			nextFundingAPR := fra.fundingRateToAPR(future.nextFundingRate)
			util.Info(fra.tag, future.name, fmt.Sprintf("next funding rate: %f", future.nextFundingRate))
			util.Info(fra.tag, future.name, fmt.Sprintf("next equivalent APR: %.2f%%", nextFundingAPR*100))
			future.consCount = 1
			for i := 1; i < len(fundingRates); i++ {
				if fundingRates[i]*future.nextFundingRate <= 0 {
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
		}
		// update profit and current hedge profit
		for name, future := range fra.futures {
			_, isStartInThisHour := fra.startFuturesInThisHour[future.name]
			// do not update profit if future just starts in this hour
			if isStartInThisHour {
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
		fra.startFuturesInThisHour = make(map[string]bool)
		// log rank
		names := fra.sortAPR()
		util.Info(fra.tag, "avgAPR Rank:")
		for _, name := range names {
			future := fra.futures[name]
			msg := fmt.Sprintf(
				"future: %s, avgAPR: %.2f%%, nextAPR: %.2f%%, consCount: %d",
				name, future.avgAPR*100, fra.fundingRateToAPR(future.nextFundingRate)*100, future.consCount)
			util.Info(fra.tag, msg)
		}
		// generate ROI report
		fra.sendTotalROIReport()
	}
}
func (fra *FRArb) updateNextFundingRates() {
	// runs every 30 sec
	updatePeriod := int64(30)
	for {
		sleepDuration := util.Duration{Second: updatePeriod}
		time.Sleep(sleepDuration.GetTimeDuration())
		now := time.Now().Unix() % 3600
		if now >= 1800 {
			util.Info(fra.tag, "updating funding rate")
			for _, future := range fra.futures {
				resp := fra.ftx.GetFutureStats(future.perpPair)
				future.nextFundingRate = resp.NextFundingRate
			}
		}
	}
}
func (fra *FRArb) Start() {
	//value, exist := os.LookupEnv("ENV")
	//isTestEnv := exist && value == "test"
	fra.createFutures()
	fra.startFuturesInThisHour = make(map[string]bool)
	go fra.updateFundingRateProfit()
	go fra.updateNextFundingRates()
	prev := time.Now()
	for {
		now := time.Now()
		duration := now.Sub(prev)
		util.Info(fra.tag, fmt.Sprintf("time used: %s", duration))
		prev = now
		for _, future := range fra.futures {
			shouldStop, shouldStart := fra.genSignal(future)
			if shouldStop {
				fra.startFuturesInThisHour[future.name] = false
			} else if shouldStart {
				fra.startFuturesInThisHour[future.name] = true
			}
		}
		//timeToNextCycle := fra.updatePeriod - time.Now().Unix()%fra.updatePeriod
		//sleepDuration := util.Duration{Second: timeToNextCycle}
		//time.Sleep(sleepDuration.GetTimeDuration())
	}
}
