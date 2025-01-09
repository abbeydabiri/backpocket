package models

import (
	"database/sql/driver"
	"encoding/json"
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

	Analysis JSONB `json:"Analysis" gorm:"type:jsonb;"`
}

type JSONB map[string]interface{}

func (j *JSONB) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, j)
}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
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
