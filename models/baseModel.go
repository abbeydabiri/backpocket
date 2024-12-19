package models

import (
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

var ILIKE = "ILIKE"

type Base struct {
	ID         uint64    `json:"ID" gorm:"primary_key;column:id;"`
	Status     string    `json:"Status" gorm:"index:"`
	Createdate time.Time `json:"Createdate" gorm:"index:"`
	Updatedate time.Time `json:"Updatedate" gorm:"index:"`
}

var counter int64

func TableID() uint64 {
	// sqlID, _ := strconv.Atoi(fmt.Sprintf("%v", time.Now().UnixNano())[:15])
	// sqlID, _ := strconv.Atoi(fmt.Sprintf("%v", time.Now().UnixNano()))

	sqlID := time.Now().UnixNano() + atomic.AddInt64(&counter, 1)
	return uint64(sqlID)
}

// BeforeCreate sets the ID and CreatedAt fields
func (base *Base) BeforeCreate(tx *gorm.DB) error {

	if base.ID == 0 {
		tx.Statement.SetColumn("ID", TableID())
	}

	if base.Createdate.IsZero() {
		tx.Statement.SetColumn("Createdate", time.Now())
	}

	if base.Updatedate.IsZero() {
		tx.Statement.SetColumn("Updatedate", time.Now())
	}

	return nil
}

// BeforeUpdate sets the UpdatedAt field
func (base *Base) BeforeUpdate(tx *gorm.DB) error {
	if base.Updatedate.IsZero() {
		tx.Statement.SetColumn("Updatedate", time.Now())
	}
	return nil
}
