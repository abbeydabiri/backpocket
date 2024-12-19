package models

import (
	"errors"

	"gorm.io/gorm"
)

type Order struct {
	Base

	Pair     string `json:"Pair" gorm:"uniqueIndex:idx_order_exchange_orderid_pair;not null"`
	Exchange string `json:"Exchange" gorm:"uniqueIndex:idx_order_exchange_orderid_pair;not null"`
	OrderID  uint64 `json:"OrderID" gorm:"uniqueIndex:idx_order_exchange_orderid_pair;not null;column:orderid"`

	Side   string `json:"Side" gorm:"index;"`
	Typeof string `json:"Typeof" gorm:"index;"`

	AutoRepeat   int    `json:"AutoRepeat" gorm:"index;column:autorepeat;"`
	AutoRepeatID uint64 `json:"AutoRepeatID" gorm:"index;column:autorepeatid;"`

	RefSide    string `json:"RefSide" gorm:"index;column:refside"`
	RefTripped string `json:"RefTripped" gorm:"index;column:reftripped"`
	RefOrderID uint64 `json:"RefOrderID" gorm:"index;column:reforderid"`
	RefEnabled int    `json:"RefEnabled" gorm:"index;column:refenabled"`

	Price      float64 `json:"Price" gorm:"index;not null"`
	Quantity   float64 `json:"Quantity" gorm:"index;not null"`
	Total      float64 `json:"Total" gorm:"index;not null"`
	Stoploss   float64 `json:"Stoploss" gorm:"index;"`
	Takeprofit float64 `json:"Takeprofit" gorm:"index;"`
}

func (model *Order) BeforeCreate(tx *gorm.DB) error {
	if err := model.Base.BeforeCreate(tx); err != nil {
		return err
	}

	if model.Pair == "" {
		return errors.New("Pair is required")
	}

	if model.Exchange == "" {
		return errors.New("Exchange is required")
	}

	if model.OrderID == 0 {
		return errors.New("OrderID is required")
	}

	if model.Price == 0 {
		return errors.New("Price is required")
	}

	if model.Quantity == 0 {
		return errors.New("Quantity is required")
	}

	if model.Total == 0 {
		return errors.New("Total is required")
	}

	return nil
}

func (model *Order) BeforeUpdate(tx *gorm.DB) error {
	if err := model.Base.BeforeUpdate(tx); err != nil {
		return err
	}

	return nil
}
