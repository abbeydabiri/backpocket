package utils

import (
	"fmt"
	"log"
	"strings"
	"time"
)

//JSONDate ...
type JSONDate time.Time

//JSONDateFormat ...
var JSONDateFormat = "2006-01-02"

//UnmarshalJSON ...
func (j *JSONDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s != "" {
		t, err := time.Parse(JSONDateFormat, s)
		if err != nil {
			log.Println(err.Error())
			return err
		}
		*j = JSONDate(t)
	}
	return nil
}

// MarshalJSON ...
func (j JSONDate) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", time.Time(j).Format(JSONDateFormat))
	return []byte(stamp), nil
	// return json.Marshal(j)
}

//Format ...
func (j JSONDate) Format() string {
	t := time.Time(j)
	return t.Format(JSONDateFormat)
}

//IsZero ...
func (j JSONDate) IsZero() bool {
	return time.Time(j).IsZero()
}

//After ...
func (j JSONDate) After(u time.Time) bool {
	return time.Time(j).After(u)
}

//Parse ...
func (j JSONDate) Parse(s string) (JSONDate, error) {
	s = strings.Trim(s, "\"")
	t, err := time.Parse(JSONDateFormat, s)
	if err != nil {
		log.Println(err.Error())
		return JSONDate(t), err
	}
	return JSONDate(t), nil
}

//ParseSpreadsheetDate ...
func (j JSONDate) ParseSpreadsheetDate(s string) (JSONDate, error) {
	s = strings.Replace(s, "\"", "", -1)
	t, err := time.Parse("02/01/2006", s)
	if err != nil {
		log.Println(err.Error())
		return JSONDate(t), err
	}
	return JSONDate(t), nil
}

// --> JSONTime is defined below

//JSONTime ...
type JSONTime time.Time

//JSONTimeFormat ...
// var JSONTimeFormat = "03:04:05 PM"
var JSONTimeFormat = "15:04:05"

//UnmarshalJSON ...
func (j *JSONTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")

	if s != "" {
		sList := strings.Split(s, ":")
		switch len(sList) {
		default:
			sList = []string{"00", "00", "00"}
		case 1:
			sList = []string{sList[0], "00", "00"}
		case 2:
			sList = []string{sList[0], sList[1], "00"}
		case 3:
		}
		s = strings.Join(sList, ":")

		t, err := time.Parse(JSONTimeFormat, s)
		if err != nil {
			log.Println(err.Error())
			return err
		}
		*j = JSONTime(t)
	}
	return nil
}

// MarshalJSON ...
func (j JSONTime) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", time.Time(j).Format(JSONTimeFormat))
	return []byte(stamp), nil
	// return json.Marshal(j)
}

//Format ...
func (j JSONTime) Format() string {
	t := time.Time(j)
	return t.Format(JSONTimeFormat)
}

//IsZero ...
func (j JSONTime) IsZero() bool {
	return time.Time(j).IsZero()
}

//Parse ...
func (j JSONTime) Parse(s string) (JSONTime, error) {
	s = strings.Trim(s, "\"")
	t, err := time.Parse(JSONTimeFormat, s)
	if err != nil {
		log.Println(err.Error())
		return JSONTime(t), err
	}
	return JSONTime(t), nil
}
