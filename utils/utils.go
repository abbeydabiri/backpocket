package utils

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

//RandomString ....
func RandomString(strlen int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, strlen)
	for i := 0; i < strlen; i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

//TruncateFloat ...
func TruncateFloat(fValue float64, fUnit int) float64 {
	sFmt := "%." + fmt.Sprintf("%v", fUnit) + "f"
	if fValue, err := strconv.ParseFloat(
		strings.TrimSpace(fmt.Sprintf(sFmt, fValue)), 64); err == nil {
		return fValue
	}
	return fValue
}
