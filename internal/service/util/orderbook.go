package util

import (
	"errors"
	"sort"
)

type Row struct {
	Price float64
	Size  float64
}
type Orderbook struct {
	Bids []Row
	Asks []Row
}

type AsksRow []Row

type BidsRow []Row

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

func (e BidsRow) Len() int {
	return len(e)
}

func (e AsksRow) Len() int {
	return len(e)
}

func (e BidsRow) Less(i, j int) bool {
	return e[i].Price > e[j].Price
}

func (e AsksRow) Less(i, j int) bool {
	return e[i].Price < e[j].Price
}

func (e BidsRow) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e AsksRow) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func SortOrderbooks(original []Row, new [][]float64, orderbookType string) *[]Row {
	var convertNewObj []Row
	for _, elem := range new {
		// Filter size = 0
		if elem[1] > 0 {
			orderbookRow := Row{elem[0], elem[1]}
			convertNewObj = append(convertNewObj, orderbookRow)
		}
	}

	original = append(original, convertNewObj...)
	if orderbookType == "bids" {
		sort.Sort(BidsRow(original))
	} else if orderbookType == "asks" {
		sort.Sort(AsksRow(original))
	}

	var result []Row
	result = append(result, original[0])
	for i := 1; i < len(original); i++ {
		if result[len(result)-1].Price == original[i].Price {
			result[len(result)-1].Size = original[i].Size
		} else {
			result = append(result, original[i])
		}
	}

	if len(result) > 50 {
		result = result[:50]
	}

	return &result
}
