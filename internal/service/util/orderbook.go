package util

import (
	"errors"
)

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

func Merge(original []Row, new [][]float64, orderbookType string) *[]Row {
	var convertNewObj []Row
	for _, elem := range new {
		// Filter size = 0
		if elem[1] > 0 {
			orderbookRow := Row{elem[0], elem[1]}
			convertNewObj = append(convertNewObj, orderbookRow)
		}
	}

	var result []Row
	originalStartIndex, newStartIndex := 0, 0
	originalLen, newLen := len(original), len(convertNewObj)

	if orderbookType == "bids" {
		for originalStartIndex < originalLen && newStartIndex < newLen {
			if original[originalStartIndex].Price == convertNewObj[newStartIndex].Price {
				result = append(result, convertNewObj[newStartIndex])
				originalStartIndex++
				newStartIndex++
			} else if original[originalStartIndex].Price > convertNewObj[newStartIndex].Price {
				result = append(result, original[originalStartIndex])
				originalStartIndex++
			} else {
				result = append(result, convertNewObj[newStartIndex])
				newStartIndex++
			}
		}
	} else if orderbookType == "asks" {
		for originalStartIndex < originalLen && newStartIndex < newLen {
			if original[originalStartIndex].Price == convertNewObj[newStartIndex].Price {
				result = append(result, convertNewObj[newStartIndex])
				originalStartIndex++
				newStartIndex++
			} else if original[originalStartIndex].Price > convertNewObj[newStartIndex].Price {
				result = append(result, convertNewObj[newStartIndex])
				newStartIndex++
			} else {
				result = append(result, original[originalStartIndex])
				originalStartIndex++
			}
		}
	}

	for originalStartIndex < originalLen {
		result = append(result, original[originalStartIndex])
		originalStartIndex++
	}

	for newStartIndex < newLen {
		result = append(result, convertNewObj[newStartIndex])
		newStartIndex++
	}

	if len(result) > 50 {
		result = result[:50]
	}

	return &result
}
