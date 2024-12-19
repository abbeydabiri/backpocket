package models

import (
	"errors"

	"gorm.io/gorm"
)

type Market struct {
	Base

	Pair     string `json:"Pair" gorm:"uniqueIndex:idx_market_exchange_pair;not null"`
	Exchange string `json:"Exchange" gorm:"uniqueIndex:idx_market_exchange_pair;not null"`

	NumOfTrades int `json:"NumOfTrades" gorm:"index; not null;column:numoftrades"`
	Closed      int `json:"Closed" gorm:"->;-:migration"` // Read-only, disable migration

	BaseAsset  string `json:"BaseAsset" gorm:"index; not null;column:baseasset"`
	QuoteAsset string `json:"QuoteAsset" gorm:"index; not null;column:quoteasset"`

	MinNotional float64 `json:"MinNotional" gorm:"column:minnotional"`
	MinQty      float64 `json:"MinQty" gorm:"column:minqty"`
	MaxQty      float64 `json:"MaxQty" gorm:"column:maxqty"`
	StepSize    float64 `json:"StepSize" gorm:"column:stepsize"`
	MinPrice    float64 `json:"MinPrice" gorm:"column:minprice"`
	MaxPrice    float64 `json:"MaxPrice" gorm:"column:maxprice"`
	TickSize    float64 `json:"TickSize" gorm:"column:ticksize"`

	Open  float64 `json:"Open" gorm:"index;"`
	Close float64 `json:"Close" gorm:"index;"`
	High  float64 `json:"High" gorm:"index;"`
	Low   float64 `json:"Low" gorm:"index;"`

	Volume      float64 `json:"Volume" gorm:"index;"`
	VolumeQuote float64 `json:"VolumeQuote" gorm:"index;column:volumequote"`
	LastPrice   float64 `json:"LastPrice" gorm:"index;column:lastprice"`
	Price       float64 `json:"Price" gorm:"index;"`

	UpperBand  float64 `json:"UpperBand" gorm:"index;column:upperband"`
	MiddleBand float64 `json:"MiddleBand" gorm:"index;column:middleband"`
	LowerBand  float64 `json:"LowerBand" gorm:"index;column:lowerband"`

	PriceChange        float64 `json:"PriceChange" gorm:"index;column:pricechange"`
	PriceChangePercent float64 `json:"PriceChangePercent" gorm:"index;column:pricechangepercent"`
	HighPrice          float64 `json:"HighPrice" gorm:"index;column:highprice"`
	LowPrice           float64 `json:"LowPrice" gorm:"index;column:lowprice"`
	RSI                float64 `json:"RSI"`
}

func (model *Market) BeforeCreate(tx *gorm.DB) error {
	if err := model.Base.BeforeCreate(tx); err != nil {
		return err
	}

	if model.Pair == "" {
		return errors.New("Pair is required")
	}

	if model.Exchange == "" {
		return errors.New("Exchange is required")
	}

	return nil
}

func (model *Market) BeforeUpdate(tx *gorm.DB) error {
	if err := model.Base.BeforeUpdate(tx); err != nil {
		return err
	}

	return nil
}
