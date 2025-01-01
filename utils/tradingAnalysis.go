package utils

import (
	"fmt"
	"math"

	"github.com/markcheno/go-talib"
)

const (
	Bullish = "Bullish"
	Bearish = "Bearish"
	Neutral = "Neutral"
)

type Candle struct {
	Close  float64
	High   float64
	Low    float64
	Open   float64
	Volume float64
}

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
	Support    float64
	Resistance float64
	Spread     float64
	Entry      float64
	StopLoss   float64
	TakeProfit float64
}

// Summary contains the final analysis report.
type summaryPattern struct {
	Chart  string
	Candle string
}
type Summary struct {
	Timeframe         string
	Trend             string
	RSI               float64
	Pattern           summaryPattern
	BollingerBands    map[string]float64
	SMA10             trendAnalysis
	SMA20             trendAnalysis
	SMA50             trendAnalysis
	RetracementLevels map[string]float64
	Candle            Candle
}

// analyzeTrend identifies the trend based on SMA and price action.
func analyzeTrend(data MarketData, period int) trendAnalysis {

	// Calculate the Simple Moving Averages.
	dataClose := data.Close[len(data.Close)-period:]

	sma := talib.Sma(dataClose, len(dataClose))
	entryPrice := sma[len(sma)-1]

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

	if len(closePrices) <= rsiPeriod {
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

// identifyCandlestickPattern detects candlestick patterns
func identifyCandlestickPattern(current, previous Candle) string {
	bodySize := math.Abs(current.Close - current.Open)
	totalRange := current.High - current.Low

	// Bullish Engulfing Pattern
	if previous.Close < previous.Open && current.Close > current.Open &&
		current.Close > previous.Open && current.Open < previous.Close {
		return "Bullish Engulfing"
	}

	// Bearish Engulfing Pattern
	if previous.Close > previous.Open && current.Close < current.Open &&
		current.Open > previous.Close && current.Close < previous.Open {
		return "Bearish Engulfing"
	}

	// Shooting Star Pattern
	if (current.High-current.Close) > 2*bodySize && bodySize < 0.2*totalRange && (current.Close-current.Low) < 0.2*totalRange {
		return "Shooting Star"
	}

	// Hammer Pattern
	if (current.Open-current.Low) > 2*bodySize && bodySize < 0.2*totalRange && (current.High-current.Close) < 0.2*totalRange {
		return "Hammer"
	}

	// Doji Pattern
	if math.Abs(current.Open-current.Close) <= 0.1*totalRange {
		return "Doji"
	}

	return ""
}

// detectChartPatterns analyzes the given price data to identify patterns
func detectChartPatterns(prices []float64) string {
	if len(prices) < 3 {
		return ""
	}

	// Detect V Pattern
	if prices[0] > prices[1] && prices[1] < prices[2] {
		return "V Pattern (Bullish Reversal)"
	}

	// Detect Inverted V Pattern
	if prices[0] < prices[1] && prices[1] > prices[2] {
		return "Inverted V Pattern (Bearish Reversal)"
	}

	// Detect Head and Shoulders Pattern
	if len(prices) >= 5 && prices[0] < prices[1] && prices[1] > prices[2] && prices[2] < prices[3] && prices[3] > prices[4] {
		return "Head and Shoulders (Bearish Reversal)"
	}

	// Detect Double Top Pattern
	if len(prices) >= 4 && math.Abs(prices[0]-prices[2]) < 0.01 && prices[1] > prices[0] && prices[3] < prices[0] {
		return "Double Top (Bearish Reversal)"
	}

	// Detect Double Bottom Pattern
	if len(prices) >= 4 && math.Abs(prices[0]-prices[2]) < 0.01 && prices[1] < prices[0] && prices[3] > prices[0] {
		return "Double Bottom (Bullish Reversal)"
	}

	return ""
}

func OverallTrend(trend10, trend20, trend50, curPrice float64) string {

	if trend10 >= trend20 && trend20 >= trend50 && curPrice >= trend10 {
		return Bullish
	}

	if trend10 <= trend20 && trend20 <= trend50 && curPrice <= trend10 {
		return Bearish
	}

	if trend50 == 0 {
		if trend10 >= trend20 && curPrice >= trend10 {
			return Bullish
		}

		if trend10 <= trend20 && curPrice <= trend10 {
			return Bearish
		}
	}

	return Neutral
}

// TimeframetTrends
func TimeframeTrends(intervals map[string]Summary, curPrice float64) string {
	trendName := ""
	totalScore := 0
	threshHold := 7
	timeWeights := map[string]int{
		"1m": 1, "3m": 1, "5m": 2, "15m": 3, "30m": 3, "4h": 5, "1d": 7,
	}
	for timeframe, interval := range intervals {
		multiplier := timeWeights[timeframe]
		trendName = OverallTrend(interval.SMA10.Entry, interval.SMA20.Entry, interval.SMA50.Entry, curPrice)
		trendScore := 0
		if trendName == Bullish {
			trendScore = 1 * multiplier
		} else if trendName == Bearish {
			trendScore = -1 * multiplier
		}
		totalScore += trendScore
	}

	if totalScore >= threshHold {
		return Bullish
	}
	if totalScore <= -threshHold {
		return Bearish
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

	candlePattern := ""
	var currentCandle, prevCandle1, prevCandle2 Candle
	chartPattern := detectChartPatterns(data.Close)

	currentCandle.Close = data.Close[len(data.Close)-1]
	currentCandle.High = data.High[len(data.High)-1]
	currentCandle.Low = data.Low[len(data.Low)-1]
	currentCandle.Open = data.Open[len(data.Open)-1]

	if len(data.Close) > 3 {
		prevCandle1.Close = data.Close[len(data.Close)-2]
		prevCandle1.High = data.High[len(data.High)-2]
		prevCandle1.Low = data.Low[len(data.Low)-2]
		prevCandle1.Open = data.Open[len(data.Open)-2]

		prevCandle2.Close = data.Close[len(data.Close)-3]
		prevCandle2.High = data.High[len(data.High)-3]
		prevCandle2.Low = data.Low[len(data.Low)-3]
		prevCandle2.Open = data.Open[len(data.Open)-3]
		candlePattern = identifyCandlestickPattern(prevCandle1, prevCandle2)
	}

	trendName := OverallTrend(analysis10.Entry, analysis20.Entry, analysis50.Entry, currentCandle.Close)
	return Summary{
		Timeframe: timeframe,
		Trend:     trendName,
		Pattern: summaryPattern{
			Chart:  chartPattern,
			Candle: candlePattern,
		},
		Candle:         currentCandle,
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
