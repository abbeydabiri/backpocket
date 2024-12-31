package utils

import (
	"fmt"

	"github.com/markcheno/go-talib"
)

const (
	Bullish = "Bullish"
	Bearish = "Bearish"
	Neutral = "Neutral"
)

// MarketData holds the historical data for analysis.
type MarketData struct {
	Close  []float64
	High   []float64
	Low    []float64
	Open   []float64
	Volume []float64
}

// trendAnalysis contains information about the market trend.
type trendAnalysis struct {
	Trend      string
	Support    float64
	Resistance float64
	Spread     float64
	Entry      float64
	StopLoss   float64
	TakeProfit float64
}

// Summary contains the final analysis report.
type Summary struct {
	Timeframe         string
	Trend             string
	RSI               float64
	BollingerBands    map[string]float64
	SMA10             trendAnalysis
	SMA20             trendAnalysis
	SMA50             trendAnalysis
	RetracementLevels map[string]float64
}

// analyzeTrend identifies the trend based on SMA and price action.
func analyzeTrend(data MarketData, period int) trendAnalysis {

	// Calculate the Simple Moving Averages.
	dataClose := data.Close[len(data.Close)-period:]

	sma := talib.Sma(dataClose, len(dataClose))
	entryPrice := sma[len(sma)-1]
	lastPrice := data.Close[len(data.Close)-1]

	trend := Neutral
	if lastPrice > entryPrice {
		trend = Bullish
	} else if lastPrice < entryPrice {
		trend = Bearish
	}

	aSupport := talib.Min(data.Low, period)
	support := aSupport[len(aSupport)-1]

	aResistance := talib.Max(data.High, period)
	resistance := aResistance[len(aResistance)-1]

	trendSpread := resistance - support
	trendSpread = TruncateFloat((trendSpread/support)*100, 3)

	entryPrice = TruncateFloat(entryPrice, 8)
	stopLoss := TruncateFloat(entryPrice-(entryPrice-support)*0.5, 8)
	takeProfit := TruncateFloat(resistance+(resistance-entryPrice)*0.5, 8)

	return trendAnalysis{
		Trend:      trend,
		Support:    support,
		Spread:     trendSpread,
		Resistance: resistance,
		Entry:      entryPrice,
		StopLoss:   stopLoss,
		TakeProfit: takeProfit,
	}
}

// calculateFibonacciRetracement computes retracement levels.
func calculateFibonacciRetracement(high, low float64) map[string]float64 {
	levels := map[string]float64{
		"0.236": TruncateFloat(high-(high-low)*0.236, 8),
		"0.382": TruncateFloat(high-(high-low)*0.382, 8),
		"0.500": TruncateFloat(high-(high-low)*0.500, 8),
		"0.618": TruncateFloat(high-(high-low)*0.618, 8),
		"0.786": TruncateFloat(high-(high-low)*0.786, 8),
	}
	return levels
}

// CalculateSmoothedRSI computes a smoothed RSI using SMA or EMA.
func CalculateSmoothedRSI(closePrices []float64, rsiPeriod int, smoothingPeriod int) float64 {
	if len(closePrices) < 2 {
		return 0
	}

	if len(closePrices) < rsiPeriod {
		rsiPeriod = len(closePrices) - 1
	}

	// Calculate standard RSI
	standardRSI := talib.Rsi(closePrices, rsiPeriod)

	// Smooth the RSI using SMA
	smoothedRSI := talib.Sma(standardRSI, smoothingPeriod)
	return TruncateFloat(smoothedRSI[len(smoothedRSI)-1], 2)
}

// calculateBollingerBands computes Bollinger Bands.
func calculateBollingerBands(closePrices []float64, period int, stdDev float64) map[string]float64 {

	middle := talib.Sma(closePrices, period)            // Middle Band (SMA)
	stdDevArray := talib.StdDev(closePrices, period, 1) // Standard Deviation

	// Calculate Upper and Lower Bands
	upper := make([]float64, len(middle))
	lower := make([]float64, len(middle))
	for i := 0; i < len(middle); i++ {
		if i >= period-1 {
			middle[i] = TruncateFloat(middle[i], 8)
			upper[i] = TruncateFloat(middle[i]+stdDev*stdDevArray[i], 8)
			lower[i] = TruncateFloat(middle[i]-stdDev*stdDevArray[i], 8)
		}
	}

	return map[string]float64{
		"upper":  upper[len(upper)-1],
		"middle": middle[len(middle)-1],
		"lower":  lower[len(lower)-1],
	}
}

func overallTrend(trend10, trend20, trend50 string) string {
	// validTrends := map[string]bool{
	// 	Bullish: true,
	// 	Bearish: true,
	// 	Neutral: true,
	// }
	// if !validTrends[trend10] || !validTrends[trend20] || !validTrends[trend50] {
	// 	return Neutral // Default to Neutral for invalid input
	// }

	trend := make(map[string]int)
	trend[trend10]++
	trend[trend20]++
	trend[trend50]++

	if trend[Bullish] >= 2 && trend[Bearish] == 0 {
		return "Strong Bullish"
	}
	if trend[Bullish] >= 2 && trend[Bearish] == 1 {
		return "Bullish"
	}
	if trend[Bearish] >= 2 && trend[Bullish] == 0 {
		return "Strong Bearish"
	}
	if trend[Bearish] >= 2 && trend[Bullish] == 1 {
		return "Bearish"
	}
	return Neutral
}

// tradingSummary creates the final analysis report.
func TradingSummary(pair, timeframe string, data MarketData) (Summary, error) {

	period10 := 10
	period20 := 20
	period50 := 50

	if len(data.Close) > 5 && len(data.Close) <= 10 {
		period10 = len(data.Close) - 1
	}
	if len(data.Close) > 10 && len(data.Close) <= 20 {
		period20 = len(data.Close) - 1
	}
	if len(data.Close) > 20 && len(data.Close) <= 50 {
		period50 = len(data.Close) - 1
	}

	if len(data.Close) < period10 {
		return Summary{}, fmt.Errorf("Not enough data for analysis of period 10")
	}

	if len(data.Close) < period20 {
		return Summary{}, fmt.Errorf("Not enough data for analysis of period 20")
	}

	var analysis10, analysis20, analysis50 trendAnalysis

	analysis20 = analyzeTrend(data, period20)
	analysis10 = analyzeTrend(data, period10)
	if len(data.Close) > period50 {
		analysis50 = analyzeTrend(data, period50)
	}

	rsiLength := 14
	smoothedRSI := CalculateSmoothedRSI(data.Close, rsiLength, 5)
	bollingerbands := calculateBollingerBands(data.Close, period20, 2)

	trendName := overallTrend(analysis10.Trend, analysis20.Trend, analysis50.Trend)
	return Summary{
		Timeframe:      timeframe,
		Trend:          trendName,
		SMA10:          analysis10,
		SMA20:          analysis20,
		SMA50:          analysis50,
		RSI:            smoothedRSI,
		BollingerBands: bollingerbands,
		RetracementLevels: calculateFibonacciRetracement(
			data.High[len(data.High)-1],
			data.Low[len(data.Low)-1]),
	}, nil
}
