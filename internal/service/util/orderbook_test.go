package util

import (
	"reflect"
	"testing"
)

var originalForBids = []Row{
	{49017, 0.3089},
	{49016, 0.3831},
	{49014, 18.35},
	{49008, 0.0547},
	{49007, 0.139},
	{49006, 0.307},
}

var originalForAsks = []Row{
	{49018, 1.3912},
	{49034, 0.2137},
	{49036, 0.1836},
	{49037, 1.0918},
	{49038, 2.0355},
	{49040, 0.6653},
	{49041, 0.82},
	{49042, 36.2009},
	{49043, 0.8139},
}

func TestMergeWhenBids(t *testing.T) {
	new := [][]float64{
		{49017, 0.3671},
		{49008, 0.007},
		{48988, 0.848},
		{48984, 0.5409},
		{48967, 36.2488},
	}

	expectation := []Row{
		{49017, 0.3671},
		{49016, 0.3831},
		{49014, 18.35},
		{49008, 0.007},
		{49007, 0.139},
		{49006, 0.307},
		{48988, 0.848},
		{48984, 0.5409},
		{48967, 36.2488},
	}

	t.Run("it should be successfully merged", func(t *testing.T) {
		if got := *Merge(originalForBids, new, "bids"); !reflect.DeepEqual(got, expectation) {
			t.Errorf("Merge() = %v, want %v", got, expectation)
		}
	})
}

func TestMergeWhenBids_WithZeroCase(t *testing.T) {
	new := [][]float64{
		{49017, 0.3671},
		{49008, 0.007},
		{48988, 0.848},
		{48984, 0},
		{48967, 36.2488},
	}

	expectation := []Row{
		{49017, 0.3671},
		{49016, 0.3831},
		{49014, 18.35},
		{49008, 0.007},
		{49007, 0.139},
		{49006, 0.307},
		{48988, 0.848},
		{48967, 36.2488},
	}

	t.Run("it should be successfully merged", func(t *testing.T) {
		if got := *Merge(originalForBids, new, "bids"); !reflect.DeepEqual(got, expectation) {
			t.Errorf("Merge() = %v, want %v", got, expectation)
		}
	})
}

func TestMergeWhenBids_WithoutSamePrice(t *testing.T) {
	new := [][]float64{
		{49002, 0.82},
		{49000, 0.24},
	}

	expectation := []Row{
		{49017, 0.3089},
		{49016, 0.3831},
		{49014, 18.35},
		{49008, 0.0547},
		{49007, 0.139},
		{49006, 0.307},
		{49002, 0.82},
		{49000, 0.24},
	}

	t.Run("it should be successfully merged", func(t *testing.T) {
		if got := *Merge(originalForBids, new, "bids"); !reflect.DeepEqual(got, expectation) {
			t.Errorf("Merge() = %v, want %v", got, expectation)
		}
	})
}

func TestMergeWhenAsks(t *testing.T) {
	new := [][]float64{
		{49018, 1.8},
		{49036, 0.15},
		{49041, 0.6189},
		{49081, 0.001},
	}

	expectation := []Row{
		{49018, 1.8},
		{49034, 0.2137},
		{49036, 0.15},
		{49037, 1.0918},
		{49038, 2.0355},
		{49040, 0.6653},
		{49041, 0.6189},
		{49042, 36.2009},
		{49043, 0.8139},
		{49081, 0.001},
	}

	t.Run("it should be successfully merged", func(t *testing.T) {
		if got := *Merge(originalForAsks, new, "asks"); !reflect.DeepEqual(got, expectation) {
			t.Errorf("Merge() = %v, want %v", got, expectation)
		}
	})
}

func TestMergeWhenAsks_WithZeroCase(t *testing.T) {
	new := [][]float64{
		{49018, 1.8},
		{49036, 0.15},
		{49040, 0},
		{49041, 0.6189},
		{49081, 0.001},
	}

	expectation := []Row{
		{49018, 1.8},
		{49034, 0.2137},
		{49036, 0.15},
		{49037, 1.0918},
		{49038, 2.0355},
		{49040, 0.6653},
		{49041, 0.6189},
		{49042, 36.2009},
		{49043, 0.8139},
		{49081, 0.001},
	}

	t.Run("it should be successfully merged", func(t *testing.T) {
		if got := *Merge(originalForAsks, new, "asks"); !reflect.DeepEqual(got, expectation) {
			t.Errorf("Merge() = %v, want %v", got, expectation)
		}
	})
}

func TestMergeWhenAsks_WithoutSamePrice(t *testing.T) {
	new := [][]float64{
		{49015, 0.08},
		{49017, 0.001},
	}

	expectation := []Row{
		{49015, 0.08},
		{49017, 0.001},
		{49018, 1.3912},
		{49034, 0.2137},
		{49036, 0.1836},
		{49037, 1.0918},
		{49038, 2.0355},
		{49040, 0.6653},
		{49041, 0.82},
		{49042, 36.2009},
		{49043, 0.8139},
	}

	t.Run("it should be successfully merged", func(t *testing.T) {
		if got := *Merge(originalForAsks, new, "asks"); !reflect.DeepEqual(got, expectation) {
			t.Errorf("Merge() = %v, want %v", got, expectation)
		}
	})
}
