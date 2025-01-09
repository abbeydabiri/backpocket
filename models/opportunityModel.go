package models

import (
	"errors"

	"gorm.io/gorm"
)

type Opportunity struct {
	Base

	Pair      string `json:"Pair" gorm:"index;not null"`
	Action    string `json:"Action" gorm:"index;"`
	Timeframe string `json:"Timeframe" gorm:"index;not null"`
	Exchange  string `json:"Exchange" gorm:"index;not null"`

	Price      float64 `json:"Price" gorm:"index;"`
	Stoploss   float64 `json:"Stoploss" gorm:"index;"`
	Takeprofit float64 `json:"Takeprofit" gorm:"index;"`

	Analysis map[string]interface{} `json:"Analysis" gorm:"type:jsonb;"`
}

func (model *Opportunity) BeforeCreate(tx *gorm.DB) error {
	if err := model.Base.BeforeCreate(tx); err != nil {
		return err
	}

	if model.Pair == "" {
		return errors.New("Pair is required")
	}

	if model.Exchange == "" {
		return errors.New("Exchange is required")
	}

	if model.Action == "" {
		return errors.New("Action is required")
	}

	return nil
}

func (model *Opportunity) BeforeUpdate(tx *gorm.DB) error {
	if err := model.Base.BeforeUpdate(tx); err != nil {
		return err
	}

	return nil
}
