package utils

import "math"

type PatternMatch struct {
	Pattern string
	Score   int
}

// Utility function to calculate slope
func slope(x1, y1, x2, y2 float64) float64 {
	return (y2 - y1) / (x2 - x1)
}

// Helper function to check approximate equality
func isApproxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

// 1. Reversal Patterns
// V Pattern (Bullish Reversal)
func isVPattern(prices []float64) bool {
	n := 5
	if len(prices) < n {
		return false
	}
	prices = prices[len(prices)-n:]

	// Check if prior trend is downward
	prevTrend := prices[n-5] > prices[n-4] && prices[n-4] > prices[n-3]
	return prevTrend && prices[n-3] > prices[n-2] && prices[n-2] < prices[n-1]
}

// Inverted V Pattern (Bearish Reversal)
func isInvertedVPattern(prices []float64) bool {
	n := 5
	if len(prices) < n {
		return false
	}
	prices = prices[len(prices)-n:]

	// Check if prior trend is upward
	prevTrend := prices[n-5] < prices[n-4] && prices[n-4] < prices[n-3]
	return prevTrend && prices[n-3] < prices[n-2] && prices[n-2] > prices[n-1]
}

// Head and Shoulders (Bearish Reversal)
func isHeadAndShoulders(prices []float64) bool {
	n := 7
	if len(prices) < n {
		return false
	}
	prices = prices[len(prices)-n:]

	// Identify key points
	point1 := prices[n-7]
	point2 := prices[n-6]
	point3 := prices[n-5]
	point4 := prices[n-4]
	point5 := prices[n-3]
	point6 := prices[n-2]
	point7 := prices[n-1]

	// Check for head and shoulders pattern
	leftShoulder := point1 < point2
	head := point3 < point2 && point3 < point4 && point4 > point2
	rightShoulder := point5 < point4 && point5 > point3 && point6 < point4 && point6 > point2
	breakdown := point7 < point5

	return leftShoulder && head && rightShoulder && breakdown
}

// Inverse Head and Shoulders (Bullish Reversal)
func isInverseHeadAndShoulders(prices []float64) bool {
	n := 7
	if len(prices) < n {
		return false
	}
	prices = prices[len(prices)-n:]

	// Identify key points
	point1 := prices[n-7]
	point2 := prices[n-6]
	point3 := prices[n-5]
	point4 := prices[n-4]
	point5 := prices[n-3]
	point6 := prices[n-2]
	point7 := prices[n-1]

	// Check for inverse head and shoulders pattern
	leftShoulder := point1 > point2
	head := point3 > point2 && point3 > point4 && point4 < point2
	rightShoulder := point5 > point4 && point5 < point3 && point6 > point4 && point6 < point2
	breakout := point7 > point5

	return leftShoulder && head && rightShoulder && breakout
}

// Double Top (Bearish Reversal)
func isDoubleTop(prices []float64) bool {
	n := 5
	if len(prices) < n {
		return false
	}
	prices = prices[len(prices)-n:]

	// Peaks must be approximately equal, with a dip between
	return isApproxEqual(prices[n-5], prices[n-3], 0.01) &&
		prices[n-4] < prices[n-5] &&
		prices[n-4] < prices[n-3]
}

// Double Bottom (Bullish Reversal)
func isDoubleBottom(prices []float64) bool {
	n := 5
	if len(prices) < n {
		return false
	}
	prices = prices[len(prices)-n:]

	// Troughs must be approximately equal, with a peak between
	return isApproxEqual(prices[n-5], prices[n-3], 0.01) &&
		prices[n-4] > prices[n-5] &&
		prices[n-4] > prices[n-3]
}

// Rising Wedge (Bearish Reversal)
func isRisingWedge(highs, lows []float64) bool {
	n := 7
	if len(highs) < n || len(lows) < n {
		return false
	}
	lows = lows[len(lows)-n:]
	highs = highs[len(highs)-n:]

	lenght := len(highs) - 1
	return slope(0, lows[0], float64(lenght), lows[lenght]) > slope(0, highs[0], float64(lenght), highs[lenght])
}

// Falling Wedge (Bullish Reversal)
func isFallingWedge(highs, lows []float64) bool {
	n := 7
	if len(highs) < n || len(lows) < n {
		return false
	}
	lows = lows[len(lows)-n:]
	highs = highs[len(highs)-n:]

	lenght := len(highs) - 1
	return slope(0, highs[0], float64(lenght), highs[lenght]) > slope(0, lows[0], float64(lenght), lows[lenght])
}

// 2. Continuation Patterns
// Flags
func isFlag(prices []float64) bool {
	n := len(prices)
	if n < 6 {
		return false
	}

	// Ensure a strong prior trend
	prevTrend := prices[n-6] < prices[n-5] && prices[n-5] < prices[n-4]

	// Check for consolidation (small range in recent candles)
	flagConsolidation := math.Abs(prices[n-3]-prices[n-2]) < 0.01 &&
		math.Abs(prices[n-2]-prices[n-1]) < 0.01

	return prevTrend && flagConsolidation
}

// Pennants
func isPennant(prices []float64) bool {
	n := len(prices)
	if n < 6 {
		return false
	}

	// Ensure strong prior trend
	priorTrend := prices[n-6] < prices[n-5] && prices[n-5] < prices[n-4]

	// Check for converging trendlines (symmetry in recent prices)
	lowerSlope := slope(0, prices[n-3], 1, prices[n-2])
	upperSlope := slope(0, prices[n-2], 1, prices[n-1])

	return priorTrend && lowerSlope > 0 && upperSlope < 0 && math.Abs(upperSlope-lowerSlope) < 0.01
}

// Rectangles
func isRectangle(prices []float64) bool {
	n := len(prices)
	if n < 6 {
		return false
	}

	// Ensure a prior trend
	priorTrend := prices[n-6] < prices[n-5] && prices[n-5] < prices[n-4]

	// Check for horizontal consolidation (within a small range)
	maxPrice := prices[n-3]
	minPrice := prices[n-3]
	for i := n - 3; i < n; i++ {
		if prices[i] > maxPrice {
			maxPrice = prices[i]
		}
		if prices[i] < minPrice {
			minPrice = prices[i]
		}
	}

	consolidation := maxPrice-minPrice < 0.02 // Adjust threshold for range

	return priorTrend && consolidation
}

// 3. Neutral Patterns
// Symmetrical Triangle
func isSymmetricalTriangle(highs, lows []float64) bool {
	if len(highs) < 3 || len(lows) < 3 {
		return false
	}
	return slope(0, highs[0], 2, highs[2]) < 0 && slope(0, lows[0], 2, lows[2]) > 0
}

// Ascending Triangle
func isAscendingTriangle(highs, lows []float64) bool {
	if len(highs) < 3 || len(lows) < 3 {
		return false
	}
	return isApproxEqual(highs[0], highs[2], 0.01) && slope(0, lows[0], 2, lows[2]) > 0
}

// Descending Triangle
func isDescendingTriangle(highs, lows []float64) bool {
	if len(highs) < 3 || len(lows) < 3 {
		return false
	}
	return isApproxEqual(lows[0], lows[2], 0.01) && slope(0, highs[0], 2, highs[2]) < 0
}
