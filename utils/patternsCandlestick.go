package utils

// Hammer: Long lower shadow and small real body
func isHammer(c Candle) bool {
	body := c.Close - c.Open
	lowerShadow := c.Open - c.Low
	return body > 0 && lowerShadow > 2*body
}

// Shooting Star: Long upper shadow and small real body
func isShootingStar(c Candle) bool {
	body := c.Close - c.Open
	upperShadow := c.High - c.Close
	return body < 0 && upperShadow > 2*-body
}

// Engulfing Bullish: Green candle engulfs red candle
func isEngulfingBullish(prev, curr Candle) bool {
	return prev.Close < prev.Open && curr.Close > curr.Open &&
		curr.Close > prev.Open && curr.Open < prev.Close
}

// Engulfing Bearish: Red candle engulfs green candle
func isEngulfingBearish(prev, curr Candle) bool {
	return prev.Close > prev.Open && curr.Close < curr.Open &&
		curr.Open > prev.Close && curr.Close < prev.Open
}

// Morning Star: Three-candle bullish reversal
func isMorningStar(last3 []Candle) bool {
	return last3[0].Close < last3[0].Open &&
		last3[1].Close < last3[1].Open &&
		last3[2].Close > last3[2].Open &&
		last3[2].Close > last3[0].Open
}

// Evening Star: Three-candle bearish reversal
func isEveningStar(last3 []Candle) bool {
	return last3[0].Close > last3[0].Open &&
		last3[1].Close > last3[1].Open &&
		last3[2].Close < last3[2].Open &&
		last3[2].Close < last3[0].Open
}
