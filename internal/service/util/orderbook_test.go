package util

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"reflect"
	"testing"
)

type BidsTestSuite struct {
	suite.Suite
	original []Row
}

type AsksTestSuite struct {
	suite.Suite
	original []Row
}

func (suite *BidsTestSuite) SetupTest() {
	suite.original = []Row{
		{49017, 0.3089},
		{49016, 0.3831},
		{49014, 18.35},
		{49008, 0.0547},
		{49007, 0.139},
		{49006, 0.307},
	}
}

func (suite *AsksTestSuite) SetupTest() {
	suite.original = []Row{
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
}

func (suite *BidsTestSuite) TestWithNormalCase() {
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

	got := *MergeOrderbook(suite.original, new, "bids")
	result := reflect.DeepEqual(got, expectation)
	assert.Equal(suite.T(), result, true)
}

func (suite *BidsTestSuite) TestWithZeroCase() {
	new := [][]float64{
		{49017, 0.3671},
		{49008, 0.007},
		{49006, 0}, // original has 48955 price
		{48988, 0.848},
		{48984, 0}, // original does not have 48955 price
		{48967, 36.2488},
		{48955, 0}, // original does not have 48955 price
		{48964, 0}, // original does not have 48955 price
	}

	expectation := []Row{
		{49017, 0.3671},
		{49016, 0.3831},
		{49014, 18.35},
		{49008, 0.007},
		{49007, 0.139},
		{48988, 0.848},
		{48967, 36.2488},
	}

	got := *MergeOrderbook(suite.original, new, "bids")
	result := reflect.DeepEqual(got, expectation)
	assert.Equal(suite.T(), result, true)
}

func (suite *BidsTestSuite) TestWithoutSamePrice() {
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

	got := *MergeOrderbook(suite.original, new, "bids")
	result := reflect.DeepEqual(got, expectation)
	assert.Equal(suite.T(), result, true)
}

func (suite *AsksTestSuite) TestWithNormalCase() {
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

	got := *MergeOrderbook(suite.original, new, "asks")
	result := reflect.DeepEqual(got, expectation)
	assert.Equal(suite.T(), result, true)
}

func (suite *AsksTestSuite) TestWithZeroCase() {
	new := [][]float64{
		{49018, 1.8},
		{49036, 0.15},
		{49040, 0}, // original has 49040 price
		{49041, 0.6189},
		{49081, 0.001},
		{49090, 0}, // original does not have 49090 price
		{48991, 0}, // original does not have 48991 price
	}

	expectation := []Row{
		{49018, 1.8},
		{49034, 0.2137},
		{49036, 0.15},
		{49037, 1.0918},
		{49038, 2.0355},
		{49041, 0.6189},
		{49042, 36.2009},
		{49043, 0.8139},
		{49081, 0.001},
	}

	got := *MergeOrderbook(suite.original, new, "asks")
	result := reflect.DeepEqual(got, expectation)
	assert.Equal(suite.T(), result, true)
}

func (suite *AsksTestSuite) TestWithoutSamePrice() {
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

	got := *MergeOrderbook(suite.original, new, "asks")
	result := reflect.DeepEqual(got, expectation)
	assert.Equal(suite.T(), result, true)
}

func TestMergeOrderbookWhenBidsTestSuite(t *testing.T) {
	suite.Run(t, new(BidsTestSuite))
}

func TestMergeOrderbookWhenAsksTestSuite(t *testing.T) {
	suite.Run(t, new(AsksTestSuite))
}
