/*
// Funding Rate Arbitrage Fork is an example of how to develop you own FRArb strategy.
*/
package character

import (
	"fmt"
	"time"

	exchange "crypto-flash/internal/service/exchange"

	util "crypto-flash/internal/service/util"
)

type FRArbFork struct {
	FRArb
}

func NewFRArbFork(ftx *exchange.FTX, notifier *Notifier, owner string, orderbooks map[string]*util.Orderbook) *FRArbFork {
	return &FRArbFork{
		FRArb: FRArb{
			SignalProvider: SignalProvider{
				tag:             "FRArbFork-" + owner,
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
			startFutureSpotSpreadRate: 0.004,
			// future spot spread should be smaller to stop position
			stopFutureSpotSpreadRate: 0,
			// previous data we used to calculate avgAPR
			prevRateDays: 7,
			// minimum USD amount to start a pair (perp + quarter)
			minAmount: 10,
			// each time we allocate this rate of balance
			freeBalanceAllocateRate: 0.33,
			// data
			futures:     make(map[string]*future),
			freeBalance: 10000,
		},
	}
}
func (fra *FRArbFork) genSignal(future *future) (bool, bool) {
	util.Error(fra.tag, "implement you strategy here")
	return false, false
}
func (fra *FRArbFork) Start() {
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
