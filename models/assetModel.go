package models

import (
	"errors"

	"gorm.io/gorm"
)

type Asset struct {
	Base

	Symbol   string `json:"Symbol" gorm:"uniqueIndex:idx_asset_exchange_symbol;not null"`
	Exchange string `json:"Exchange" gorm:"uniqueIndex:idx_asset_exchange_symbol;not null"`

	Address string `json:"Address" gorm:"index;"`
	State   string `json:"State" gorm:"index;"`

	Locked float64 `json:"Locked" gorm:"index;"`
	Free   float64 `json:"Free" gorm:"index;"`
}

func (model *Asset) BeforeCreate(tx *gorm.DB) error {
	if err := model.Base.BeforeCreate(tx); err != nil {
		return err
	}

	if model.Symbol == "" {
		return errors.New("Symbol is required")
	}

	if model.Exchange == "" {
		return errors.New("Exchange is required")
	}

	return nil
}

func (model *Asset) BeforeUpdate(tx *gorm.DB) error {
	if err := model.Base.BeforeUpdate(tx); err != nil {
		return err
	}

	return nil
}
