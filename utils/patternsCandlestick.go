package utils

import (
	"fmt"
	"math"
	"strings"
)

// calculatePrecision dynamically determines the precision based on the smallest value increment in the candlestick.
func calculatePrecision(numbers ...float64) (smallestUnit float64) {
	largestDecimals := 0

	for _, number := range numbers {
		// Convert the number to a string representation
		numberStr := fmt.Sprintf("%.10f", number) // Up to 18 decimal places for precision
		// Remove trailing zeros
		numberStr = strings.TrimRight(numberStr, "0")
		// Split into integer and decimal parts
		parts := strings.Split(numberStr, ".")
		if len(parts) == 2 { // Has decimal part
			decimals := len(parts[1])
			if decimals > largestDecimals {
				largestDecimals = decimals
			}
		}
	}
	largestDecimals--

	// Calculate smallest unit as 1 / 10^largestDecimals
	smallestUnit = math.Pow(10, -float64(largestDecimals))

	return smallestUnit
}

// - one candle stick patterns - //

// isBearishMarubozu checks for a Bearish Marubozu pattern (Open is at High, Close is at Low).
func isBearishMarubozu(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	return math.Abs(c.Open-c.High) <= precision && math.Abs(c.Close-c.Low) <= precision
}

// isBullishMarubozu checks for a Bullish Marubozu pattern (Open is at Low, Close is at High).
func isBullishMarubozu(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	return math.Abs(c.Open-c.Low) <= precision && math.Abs(c.Close-c.High) <= precision
}

// isBearishSpinningTop checks for a Bearish Spinning Top pattern (Small Body, Larger Shadows, Close < Open).
func isBearishSpinningTop(c Candle) bool {
	body := math.Abs(c.Open - c.Close)
	upperShadow := c.High - math.Max(c.Open, c.Close)
	lowerShadow := math.Min(c.Open, c.Close) - c.Low
	return c.Close < c.Open && body <= upperShadow && body <= lowerShadow
}

// isBullishSpinningTop checks for a Bullish Spinning Top pattern (Small Body, Larger Shadows, Close > Open).
func isBullishSpinningTop(c Candle) bool {
	body := math.Abs(c.Open - c.Close)
	upperShadow := c.High - math.Max(c.Open, c.Close)
	lowerShadow := math.Min(c.Open, c.Close) - c.Low
	return c.Close > c.Open && body <= upperShadow && body <= lowerShadow
}

// isNormalDoji checks for a Normal Doji pattern.
func isNormalDoji(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	return math.Abs(c.Open-c.Close) <= precision && (c.High-c.Low) > 2*(math.Abs(c.Open-c.Close))
}

// isDragonflyDoji checks for a Dragonfly Doji pattern.
func isDragonflyDoji(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	return math.Abs(c.Open-c.Close) <= precision && (c.High-c.Low) > 2*(c.High-c.Open) && (c.High-c.Close) > 2*(c.High-c.Low)
}

// isFourPriceDoji checks for a Four Price Doji pattern.
func isFourPriceDoji(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	return math.Abs(c.Open-c.High) <= precision && math.Abs(c.High-c.Low) <= precision && math.Abs(c.Low-c.Close) <= precision
}

// isGravestoneDoji checks for a Gravestone Doji pattern.
func isGravestoneDoji(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	return math.Abs(c.Open-c.Close) <= precision && (c.High-c.Low) > 2*(c.Open-c.Low) && (c.High-c.Close) > 2*(c.High-c.Low)
}

// isLongLeggedDoji checks for a Long-Legged Doji pattern.
func isLongLeggedDoji(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	return math.Abs(c.Open-c.Close) <= precision && (c.High-c.Low) > 4*(math.Abs(c.Open-c.Close))
}

// isBullishHammer checks for a Bullish Hammer pattern (small body near high, long lower shadow).
func isBullishHammer(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	body := math.Abs(c.Open - c.Close)
	lowerShadow := math.Min(c.Open, c.Close) - c.Low
	upperShadow := c.High - math.Max(c.Open, c.Close)
	return lowerShadow > 2*body && upperShadow <= precision && c.Close > c.Open
}

// isBullishInvertedHammer checks for a Bullish Inverted Hammer pattern (small body near low, long upper shadow).
func isBullishInvertedHammer(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	body := math.Abs(c.Open - c.Close)
	upperShadow := c.High - math.Max(c.Open, c.Close)
	lowerShadow := math.Min(c.Open, c.Close) - c.Low
	return upperShadow > 2*body && lowerShadow <= precision && c.Close > c.Open
}

// isBearishHangingMan checks for a Bearish Hanging Man pattern (small body near high, long lower shadow).
func isBearishHangingMan(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	body := math.Abs(c.Open - c.Close)
	lowerShadow := math.Min(c.Open, c.Close) - c.Low
	upperShadow := c.High - math.Max(c.Open, c.Close)
	return lowerShadow > 2*body && upperShadow <= precision && c.Close < c.Open
}

// isBearishShootingStar checks for a Bearish Shooting Star pattern (small body near low, long upper shadow).
func isBearishShootingStar(c Candle) bool {
	precision := calculatePrecision(c.Open, c.High, c.Low, c.Close)
	body := math.Abs(c.Open - c.Close)
	upperShadow := c.High - math.Max(c.Open, c.Close)
	lowerShadow := math.Min(c.Open, c.Close) - c.Low
	return upperShadow > 2*body && lowerShadow <= precision && c.Close < c.Open
}

// - two candle stick patterns - //

// Engulfing Bullish: Green candle engulfs red candle
func isBullishEngulfing(prev, curr Candle) bool {
	return prev.Close < prev.Open && curr.Close > curr.Open &&
		curr.Close > prev.Open && curr.Open < prev.Close
}

// Engulfing Bearish: Red candle engulfs green candle
func isBearishEngulfing(prev, curr Candle) bool {
	return prev.Close > prev.Open && curr.Close < curr.Open &&
		curr.Open > prev.Close && curr.Close < prev.Open
}

// isBullishTweezerBottoms checks for a Bullish Tweezer Bottoms pattern (two candles with the same low, second candle bullish).
func isBullishTweezerBottoms(prev, curr Candle) bool {
	precision := calculatePrecision(prev.Open, prev.High, prev.Low, prev.Close)
	bodyPrev := math.Abs(prev.Open - prev.Close)
	bodyCurr := math.Abs(curr.Open - curr.Close)
	lowerShadowPrev := prev.Low - math.Min(prev.Open, prev.Close)
	lowerShadowCurr := curr.Low - math.Min(curr.Open, curr.Close)
	return prev.Close < prev.Open && curr.Close > curr.Open &&
		math.Abs(prev.Low-curr.Low) <= precision &&
		lowerShadowPrev > 2*bodyPrev && lowerShadowCurr > 2*bodyCurr
}

// isBearishTweezerBottoms checks for a Bearish Tweezer Bottoms pattern (two candles with the same low, second candle bearish).
func isBearishTweezerTops(prev, curr Candle) bool {
	precision := calculatePrecision(prev.Open, prev.High, prev.Low, prev.Close)
	bodyPrev := math.Abs(prev.Open - prev.Close)
	bodyCurr := math.Abs(curr.Open - curr.Close)
	upperShadowPrev := prev.High - math.Max(prev.Open, prev.Close)
	upperShadowCurr := curr.High - math.Max(curr.Open, curr.Close)
	return prev.Close > prev.Open && curr.Close < curr.Open &&
		math.Abs(prev.High-curr.High) <= precision &&
		upperShadowPrev > 2*bodyPrev && upperShadowCurr > 2*bodyCurr
}

// - three candle stick patterns - //
// Three White Soldiers: Each candlestick opens within the body of the preceding candlestick and closes beyond its high price.
func isBullishThreeWhiteSoldiers(last3 []Candle) bool {
	return len(last3) == 3 &&
		last3[0].Close > last3[0].Open &&
		last3[1].Close > last3[1].Open &&
		last3[2].Close > last3[2].Open &&
		last3[1].Open < last3[0].Close &&
		last3[2].Open < last3[1].Close
}

// Bullish Deliberation: Two rising tall green candles, with partial overlap and each close near the high, followed by a small green candle that opens near the preceding close.
func isBullishDeliberation(last3 []Candle) bool {
	return last3[0].Close > last3[0].Open &&
		last3[1].Close > last3[1].Open &&
		last3[2].Close > last3[2].Open &&
		last3[0].Close < last3[1].Open &&
		last3[1].Close < last3[2].Open &&
		math.Abs(last3[2].Close-last3[2].Open) < math.Abs(last3[0].Close-last3[0].Open) &&
		math.Abs(last3[2].Close-last3[2].Open) < math.Abs(last3[1].Close-last3[1].Open)
}

// Bearish Identical Three Crows: Three identical falling red candles with no overlap (between the bodies) and each close near the low.
func isBearishIdenticalThreeCrows(last3 []Candle) bool {
	return last3[0].Close < last3[0].Open &&
		last3[1].Close < last3[1].Open &&
		last3[2].Close < last3[2].Open &&
		last3[0].Close < last3[1].Open &&
		last3[1].Close < last3[2].Open &&
		math.Abs(last3[0].Close-last3[0].Low) <= calculatePrecision(last3[0].Open, last3[0].High, last3[0].Low, last3[0].Close) &&
		math.Abs(last3[1].Close-last3[1].Low) <= calculatePrecision(last3[1].Open, last3[1].High, last3[1].Low, last3[1].Close) &&
		math.Abs(last3[2].Close-last3[2].Low) <= calculatePrecision(last3[2].Open, last3[2].High, last3[2].Low, last3[2].Close)
}

// Three Black Crows: Each candlestick opens within the body of the preceding candlestick and closes beyond its low price
func isBearishThreeBlackCrows(last3 []Candle) bool {
	return last3[0].Close < last3[0].Open &&
		last3[1].Close < last3[1].Open &&
		last3[2].Close < last3[2].Open &&
		last3[1].Open < last3[0].Close &&
		last3[2].Open < last3[1].Close
}

// Morning Star: Three candle pattern, first bearish, second small-bodied, third bullish
func isBullishMorningStar(last3 []Candle) bool {
	return last3[0].Close < last3[0].Open &&
		math.Abs(last3[1].Open-last3[1].Close) < math.Abs(last3[0].Open-last3[0].Close) &&
		last3[2].Close > last3[2].Open &&
		last3[2].Close > (last3[0].Open-last3[0].Close)/2
}

// Evening Star: Three candle pattern, first bullish, second small-bodied, third bearish
func isBearishEveningStar(last3 []Candle) bool {
	return last3[0].Close > last3[0].Open &&
		math.Abs(last3[1].Open-last3[1].Close) < math.Abs(last3[0].Close-last3[0].Open) &&
		last3[2].Close < last3[2].Open &&
		last3[2].Close < (last3[0].Close-last3[0].Open)/2
}

// - four candle stick patterns - //

// Bearish Concealing Baby Swallow: This rare pattern consists four red candles. Two consecutive tall red candles with no shadows gap down to a third tall red candle with a tall upper shadow (that overlaps the preceding body) and no lower shadow. This is followed by a fourth red candle which completely engulfs the previous candle (including the shadow).
func isBearishConcealingBabySwallow(last4 []Candle) bool {
	return len(last4) == 4 &&
		last4[0].Close < last4[0].Open &&
		last4[1].Close < last4[1].Open &&
		last4[2].Close < last4[2].Open &&
		last4[3].Close < last4[3].Open &&
		math.Abs(last4[0].High-last4[0].Open) <= calculatePrecision(last4[0].Open, last4[0].High, last4[0].Low, last4[0].Close) &&
		math.Abs(last4[0].Low-last4[0].Close) <= calculatePrecision(last4[0].Open, last4[0].High, last4[0].Low, last4[0].Close) &&
		math.Abs(last4[1].High-last4[1].Open) <= calculatePrecision(last4[1].Open, last4[1].High, last4[1].Low, last4[1].Close) &&
		math.Abs(last4[1].Low-last4[1].Close) <= calculatePrecision(last4[1].Open, last4[1].High, last4[1].Low, last4[1].Close) &&
		last4[2].High > last4[1].Close &&
		math.Abs(last4[2].Low-last4[2].Close) <= calculatePrecision(last4[2].Open, last4[2].High, last4[2].Low, last4[2].Close) &&
		last4[3].Open > last4[2].Close &&
		last4[3].Close < last4[2].Open &&
		last4[3].High > last4[2].High &&
		last4[3].Low < last4[2].Low
}

// Bearish Three Line Strike: Three falling red candles, with lower closes, followed by a tall green candle that opens below (or equal to) the preceding close and closes above the bodies of the preceding three candles.
func isBearishThreeLineStrike(last4 []Candle) bool {
	return len(last4) == 4 &&
		last4[0].Close < last4[0].Open &&
		last4[1].Close < last4[1].Open &&
		last4[2].Close < last4[2].Open &&
		last4[3].Close > last4[3].Open &&
		last4[1].Close < last4[0].Close &&
		last4[2].Close < last4[1].Close &&
		last4[3].Open <= last4[2].Close &&
		last4[3].Close > last4[0].Open
}

// Bullish Three Line Strike: Three rising green candles, with higher closes, followed by a tall red candle that opens above (or equal to) the preceding close and closes below the bodies of the preceding three candles.
func isBullishThreeLineStrike(last4 []Candle) bool {
	return len(last4) == 4 &&
		last4[0].Close > last4[0].Open &&
		last4[1].Close > last4[1].Open &&
		last4[2].Close > last4[2].Open &&
		last4[3].Close < last4[3].Open &&
		last4[1].Close > last4[0].Close &&
		last4[2].Close > last4[1].Close &&
		last4[3].Open >= last4[2].Close &&
		last4[3].Close < last4[0].Open
}
