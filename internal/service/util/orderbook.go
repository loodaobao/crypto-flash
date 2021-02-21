package util

import "errors"

type Row struct {
	Price float64
	Size  float64
}
type Orderbook struct {
	Bids []Row
	Asks []Row
}

func (ob *Orderbook) Add(side string, price, size float64) {
	row := Row{Price: price, Size: size}
	if side == "bid" {
		ob.Bids = append(ob.Bids, row)
	} else if side == "ask" {
		ob.Asks = append(ob.Asks, row)
	}
}
func (ob *Orderbook) GetMarketBuyPrice() (float64, error) {
	if len(ob.Asks) < 1 {
		return 0, errors.New("No available ask orderbook")
	}
	return ob.Asks[0].Price, nil
}
func (ob *Orderbook) GetMarketSellPrice() (float64, error) {
	if len(ob.Bids) < 1 {
		return -1, errors.New("No available bid orderbook")
	}
	return ob.Bids[0].Price, nil
}
