/*
// Funding Rate Arbitrage is a signal provider utilizes funding rate on
// perpetual contract to earn profit.
// TODO: all
*/
package character

import (
	"fmt"
	"math"
	"sort"
	"time"

	exchange "crypto-flash/internal/service/exchange"

	util "crypto-flash/internal/service/util"
)

type future struct {
	name              string
	fundingRates      []float64
	consCount         int
	estAPR            float64
	size              float64
	totalProfit       float64
	perpEnterPrice    float64
	quarterEnterPrice float64
}
type FRArb struct {
	SignalProvider
	ftx *exchange.FTX
	// strategy config
	quarterContractName string
	updatePeriod        int64
	reportPeriod        int64
	futureNames         []string
	leverage            float64
	longTime            int
	startAPRThreshold   float64
	prevRateDays        int64
	minAmount           float64
	// data
	freeBalance  float64
	futures      map[string]*future
	startFutures []*future
	stopFutures  []*future
}

func NewFRArb(ftx *exchange.FTX, notifier *Notifier) *FRArb {
	return &FRArb{
		SignalProvider: SignalProvider{
			tag:             "FRArbProvider",
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
		reportPeriod: 1,
		futureNames: []string{
			"BTC", "ETH", "LTC", "LINK", "DOGE", "ADA", "AAVE", "XRP", "DOT", "SUSHI",
			"DEFI", "BCH", "1INCH", "BNB", "EOS", "UNI", "ATOM", "YFI",
			"GRT", "ALGO", "THETA", "SXP", "COMP", "TRX", "SOL",
			"BSV", "XTZ", "BRZ", "FIL",
			"BAL", "OMG", "WAVES", "AVAX",
			"SHIT", "ALT", "CREAM", "EXCH", "MID", "USDT",
			"DRGN", "OKB",
			"PRIV", "ONT", "TRU", "UNISWAP", "BTMX",
			"CHZ", "XAUT", "LEO", "TRYB", "PAXG", "CUSDT",
			// new "BADGER", "PERP", "LINA", "BAO"
			// pair without quarter: "FTT", "ALPHA", "YFII", "ZEC", "REN", "HOLY", "BAT", "MKR", "MATIC", "VET"
			// "XMR", "HT", "CRV", "RUNE", "TOMO", "KNC", "MTA", "ETC", "NEO", "EGLD", "BTT", "SECO", "FLM"
			// "SNX", "DMG", "RSR", "HNT", "KSM", "XLM", "BAND", "AMPL"
		},
		// perp and quarter have 1/2 pairPortion and leverage
		leverage: 5,
		// consecutive hours of positive/negative funding rate
		longTime: 1 * 24,
		// start arbitrage if APR is more then this threshold
		startAPRThreshold: 5,
		// previous data we used to calculate estAPR
		prevRateDays: 7,
		// minimum USD amount to start a pair (perp + quarter)
		minAmount: 10,
		// data
		futures:     make(map[string]*future),
		freeBalance: 1000000,
	}
}

func (fra *FRArb) getFutureName(future string, isPerp bool) string {
	if isPerp {
		return future + "-PERP"
	} else {
		return future + "-" + fra.quarterContractName
	}
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

func (fra *FRArb) genSignal(future *future) {
	fundingRates := future.fundingRates
	nextFundingRate := fundingRates[0]
	nextFundingAPR := math.Abs(fundingRates[0]) * 365 * 24 * fra.leverage / 2
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
	future.estAPR = totalRate * toAnnual * fra.leverage / 2
	util.Info(fra.tag, future.name, fmt.Sprintf("estAPR: %.2f%%", future.estAPR*100))
	notProfitable := (future.size * nextFundingRate) > 0
	if notProfitable {
		util.Info(fra.tag, "not profitable: "+future.name)
		fra.broadcast("not profitable: " + future.name)
		fra.stopFutures = append(fra.stopFutures, future)
	} else if nextFundingAPR >= fra.startAPRThreshold && future.size == 0 {
		util.Info(fra.tag, "profitable: "+future.name)
		fra.broadcast("profitable: " + future.name + "\n" + fmt.Sprintf("estAPR: %.2f%%", future.estAPR*100))
		// check future has quarterContract
		// TODO: use spot
		_, err := fra.ftx.GetFuture(fra.getFutureName(future.name, false))
		if err != nil {
			return
		}
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
		kvs = append(kvs, kv{name, future.estAPR})
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

func (fra *FRArb) sendROIReport() {
	if fra.notifier == nil {
		return
	}
	msg := "Report\n"
	runTime := time.Now().Sub(fra.startTime)
	d := util.FromTimeDuration(runTime)
	msg += "Runtime: " + d.String() + "\n\n"
	names := fra.sortAPR()
	totalProfit := 0.0
	for _, name := range names {
		future := fra.futures[name]
		if future.totalProfit == 0 {
			continue
		}
		msg += "future: " + future.name + "\n"
		msg += fmt.Sprintf("estAPR: %.2f%%\n", future.estAPR*100)
		msg += fmt.Sprintf("consCount: %d\n", future.consCount)
		msg += fmt.Sprintf("next funding rate: %f\n", future.fundingRates[0])
		msg += fmt.Sprintf("size: %f\n", future.size)
		msg += fmt.Sprintf("future profit: %f\n\n", future.totalProfit)
		totalProfit += future.totalProfit
	}
	msg += fmt.Sprintf("Total Profit: %.2f\n", totalProfit)
	balance := fra.initBalance + totalProfit
	roi := util.CalcROI(fra.initBalance, balance)
	msg += fmt.Sprintf("ROI: %.2f%%\n", roi*100)
	ar := util.CalcAnnualFromROI(roi, runTime.Seconds())
	msg += fmt.Sprintf("Annualized Return: %.2f%%", ar*100)

	util.Info(fra.tag, msg)
	fra.broadcast(msg)
}

func (fra *FRArb) startPair(future *future, ratio float64) {
	perpSide := "long"
	//quarterSide := "short"
	if future.size < 0 {
		// long pays short, short perp, long quarter
		perpSide = "short"
		//quarterSide = "long"
	}
	/*
		// TODO: set stop loss
		fra.sendSignal(&util.Signal{
			Market: fra.getFutureName(future.name, true),
			Side: perpSide,
			Reason: "Profitable",
			Ratio: ratio,
		})
		fra.sendSignal(&util.Signal{
			Market: fra.getFutureName(future.name, false),
			Side: quarterSide,
			Reason: "Profitable",
			Ratio: ratio,
		})*/
	perpPrices, err := fra.ftx.GetFuture(fra.getFutureName(future.name, true))
	if err != nil {
		return
	}
	quarterPrices, err := fra.ftx.GetFuture(fra.getFutureName(future.name, false))
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
	fra.broadcast(fmt.Sprintf("start earning on %s, size %f", future.name, future.size))
}
func (fra *FRArb) calculateHedgeProfit(future *future) (float64, error) {
	perpPrices, err := fra.ftx.GetFuture(fra.getFutureName(future.name, true))
	if err != nil {
		return 0, err
	}
	quarterPrices, err := fra.ftx.GetFuture(fra.getFutureName(future.name, false))
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
func (fra *FRArb) stopPair(future *future) {
	/*
		fra.sendSignal(&util.Signal{
			Market: fra.getFutureName(future.name, true),
			Side: "close",
			Reason: "Not profitable",
		})
		fra.sendSignal(&util.Signal{
			Market: fra.getFutureName(future.name, false),
			Side: "close",
			Reason: "Not profitable",
		})*/
	hedgeProfit, err := fra.calculateHedgeProfit(future)
	if err != nil {
		util.Error("cannot calculate hedge profit")
	}
	future.totalProfit -= math.Abs(future.size) * fra.ftx.Fee * 2
	future.totalProfit += hedgeProfit
	util.Info(fra.tag, fmt.Sprintf("stop earning on %s, size %f", future.name, future.size))
	util.Info(fra.tag, fmt.Sprintf("hedge profit: %f", hedgeProfit))
	fra.broadcast(fmt.Sprintf("stop earning on %s, size %f", future.name, future.size))
	fra.broadcast(fmt.Sprintf("hedge profit: %f", hedgeProfit))
	pairPortion := math.Abs(future.size) / fra.leverage * 2
	fra.freeBalance += pairPortion
	future.size = 0
}
func (fra *FRArb) sendHedgeProfitReport() {
	for name, future := range fra.futures {
		if future.size != 0 {
			hedgeProfit, err := fra.calculateHedgeProfit(future)
			if err != nil {
				util.Error("cannot calculate hedge profit")
			}
			hedgeProfitROI := hedgeProfit / math.Abs(future.size)
			msg := fmt.Sprintf("hedge profit on future %s: %f, %.2f%%", name, hedgeProfit, hedgeProfitROI*100)
			util.Info(fra.tag, msg)
			fra.broadcast(msg)
		}
	}
}
func (fra *FRArb) Start() {
	// get previous funding rate
	now := time.Now().Unix()
	end := now - now%(60*60)
	start := end - fra.prevRateDays*24*60*60
	for _, name := range fra.futureNames {
		fra.futures[name] = &future{
			name: name,
		}
		fra.futures[name].fundingRates = fra.ftx.GetFundingRates(start, end, fra.getFutureName(name, true))
	}
	for {
		now = time.Now().Unix()
		// TODO: check existing position every updatePeriod
		// one hour and 15 second just passed, get next funding rate
		getFundingRateOffset := fra.updatePeriod
		if now%(60*60) == getFundingRateOffset {
			for name, future := range fra.futures {
				resp := fra.ftx.GetFutureStats(fra.getFutureName(name, true))
				nextFundingRate := resp.NextFundingRate
				future.fundingRates = append([]float64{nextFundingRate}, future.fundingRates[:24*fra.prevRateDays-1]...)
				// calculate profit if future has position
				if future.size != 0 {
					profit := future.size * future.fundingRates[1] * -1
					future.totalProfit += profit
					msg := fmt.Sprintf("earning %.2f USD on %s", profit, name)
					util.Info(fra.tag, msg)
					fra.broadcast(msg)
				}
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
					if future.fundingRates[0] > 0 {
						future.size = -size
					} else {
						future.size = size
					}
					fra.startPair(future, 1/startFutureCount/2*fra.leverage)
				}
				fra.freeBalance = 0
			}
			fra.startFutures = fra.startFutures[:0]
			fra.stopFutures = fra.stopFutures[:0]
			names := fra.sortAPR()
			util.Info(fra.tag, "estAPR Rank:")
			for _, name := range names {
				future := fra.futures[name]
				msg := fmt.Sprintf(
					"future: %s, estAPR: %.2f%%, nextAPR: %.2f%%, consCount: %d",
					name, future.estAPR*100, math.Abs(future.fundingRates[0])*24*365*fra.leverage/2*100, future.consCount)
				util.Info(fra.tag, msg)
			}
			fra.sendHedgeProfitReport()
		}
		// generate roi report
		if now%(fra.reportPeriod*60*60) == getFundingRateOffset {
			fra.sendROIReport()
		}
		timeToNextCycle := fra.updatePeriod - time.Now().Unix()%fra.updatePeriod
		sleepDuration := util.Duration{Second: timeToNextCycle}
		time.Sleep(sleepDuration.GetTimeDuration())
	}
}
