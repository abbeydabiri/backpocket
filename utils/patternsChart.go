package utils

import "math"

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
	n := len(prices)
	if n < 3 {
		return false
	}
	return prices[n-3] > prices[n-2] && prices[n-2] < prices[n-1]
}

// Inverted V Pattern (Bearish Reversal)
func isInvertedVPattern(prices []float64) bool {
	n := len(prices)
	if n < 3 {
		return false
	}
	return prices[n-3] < prices[n-2] && prices[n-2] > prices[n-1]
}

// Head and Shoulders (Bearish Reversal)
func isHeadAndShoulders(prices []float64) bool {
	n := len(prices)
	if n < 5 {
		return false
	}
	return prices[n-5] < prices[n-4] && prices[n-3] > prices[n-4] && prices[n-1] < prices[n-3]
}

// Inverse Head and Shoulders (Bullish Reversal)
func isInverseHeadAndShoulders(prices []float64) bool {
	n := len(prices)
	if n < 5 {
		return false
	}
	return prices[n-5] > prices[n-4] && prices[n-3] < prices[n-4] && prices[n-1] > prices[n-3]
}

// Double Top (Bearish Reversal)
func isDoubleTop(prices []float64) bool {
	n := len(prices)
	if n < 4 {
		return false
	}
	return isApproxEqual(prices[n-4], prices[n-2], 0.01) && prices[n-3] < prices[n-4]
}

// Double Bottom (Bullish Reversal)
func isDoubleBottom(prices []float64) bool {
	n := len(prices)
	if n < 4 {
		return false
	}
	return isApproxEqual(prices[n-4], prices[n-2], 0.01) && prices[n-3] > prices[n-4]
}

// Rising Wedge (Bearish Reversal)
func isRisingWedge(highs, lows []float64) bool {
	if len(highs) < 3 || len(lows) < 3 {
		return false
	}
	return slope(0, lows[0], 2, lows[2]) > slope(0, highs[0], 2, highs[2])
}

// Falling Wedge (Bullish Reversal)
func isFallingWedge(highs, lows []float64) bool {
	if len(highs) < 3 || len(lows) < 3 {
		return false
	}
	return slope(0, highs[0], 2, highs[2]) > slope(0, lows[0], 2, lows[2])
}

// 2. Continuation Patterns
// Flags
func isFlag(prices []float64) bool {
	n := len(prices)
	if n < 4 {
		return false
	}
	return slope(0, prices[0], float64(n/2), prices[n/2]) > 0 &&
		isApproxEqual(slope(float64(n/2), prices[n/2], float64(n-1), prices[n-1]), 0, 0.01)
}

// Pennants
func isPennant(highs, lows []float64) bool {
	if len(highs) < 3 || len(lows) < 3 {
		return false
	}
	return slope(0, lows[0], 2, lows[2]) > slope(0, highs[0], 2, highs[2])
}

// Rectangles
func isRectangle(prices []float64) bool {
	n := len(prices)
	if n < 4 {
		return false
	}
	return isApproxEqual(prices[0], prices[n-1], 0.01) &&
		isApproxEqual(prices[n/2], prices[n-1], 0.01)
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
