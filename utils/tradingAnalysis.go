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
type SummaryPattern struct {
	Chart  string
	Candle string
}
type Summary struct {
	Timeframe         string
	Trend             string
	RSI               float64
	Pattern           SummaryPattern
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

	dataLow := data.Low[len(data.Low)-period:]
	aSupport := talib.Min(dataLow, period)
	support := aSupport[len(aSupport)-1]

	dataHigh := data.High[len(data.High)-period:]
	aResistance := talib.Max(dataHigh, period)
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
func identifyCandlestickPattern(candles []Candle) string {
	if len(candles) < 4 {
		return "Candles less than 4"
	}

	latest := candles[len(candles)-1]
	penultimate := candles[len(candles)-2]

	// - four candle stick patterns - //
	// Bearish Concealing Baby Swallow
	if isBearishConcealingBabySwallow(candles[len(candles)-4:]) {
		return "Bearish: Concealing Baby"
	}

	//Bearish Three Line Strike
	if isBearishThreeLineStrike(candles[len(candles)-4:]) {
		return "Bearish: Three Line Strike"
	}

	//Bullish Three Line Strike
	if isBullishThreeLineStrike(candles[len(candles)-4:]) {
		return "Bullish: Three Line Strike"
	}

	// - three candle stick patterns - //
	// Bullish Deliberation (Variation of Three White Soldiers)
	if isBullishDeliberation(candles[len(candles)-3:]) {
		return "Bullish: Deliberation"
	}

	// Bullish Three White Soldiers
	if isBullishThreeWhiteSoldiers(candles[len(candles)-3:]) {
		return "Bullish: Three White Soldiers"
	}

	// Bearish Identical Three Crows
	if isBearishIdenticalThreeCrows(candles[len(candles)-3:]) {
		return "Bearish: Identical Three Crows"
	}

	// Bearish Three Black Crows
	if isBearishThreeBlackCrows(candles[len(candles)-3:]) {
		return "Bearish: Three Black Crows"
	}

	// Bullish Morning Star
	if isBullishMorningStar(candles[len(candles)-3:]) {
		return "Bullish: Morning Star"
	}

	// Bearish Evening Star
	if isBearishEveningStar(candles[len(candles)-3:]) {
		return "Bearish: Evening Star"
	}

	// - two candle stick patterns - //

	// Bullish Engulfing
	if isBullishEngulfing(penultimate, latest) {
		return "Bullish: Engulfing"
	}

	// Bearish Engulfing
	if isBearishEngulfing(penultimate, latest) {
		return "Bearish: Engulfing"
	}

	//Bullish Tweezer Bottoms
	if isBullishTweezerBottoms(penultimate, latest) {
		return "Bullish: Tweezer Bottoms"
	}

	//Bearish Tweezer Tops
	if isBearishTweezerTops(penultimate, latest) {
		return "Bearish: Tweezer Tops"
	}

	// - one candle stick patterns - //
	// Bullish Marubozu
	if isBullishMarubozu(latest) {
		return "Bullish: Marubozu"
	}

	// Bearish Marubozu
	if isBearishMarubozu(latest) {
		return "Bearish: Marubozu"
	}

	// Normal Doji
	if isNormalDoji(latest) {
		return "Normal Doji"
	}

	// Dragonfly Doji
	if isDragonflyDoji(latest) {
		return "Dragonfly Doji"
	}

	// Four Price Doji
	if isFourPriceDoji(latest) {
		return "Four Price Doji"
	}

	// Gravestone Doji
	if isGravestoneDoji(latest) {
		return "Gravestone Doji"
	}

	//Long Legged Doji
	if isLongLeggedDoji(latest) {
		return "Long Legged Doji"
	}

	// Bullish Hammer
	if isBullishHammer(latest) {
		return "Bullish: Hammer"
	}

	// Bullish Inverted Hammer
	if isBullishInvertedHammer(latest) {
		return "Bullish: Inverted Hammer"
	}

	// Bearish: Hanging Man
	if isBearishHangingMan(latest) {
		return "Bearish: Hanging Man"
	}

	// Bearish: Shooting Star
	if isBearishShootingStar(latest) {
		return "Bearish: Shooting Star"
	}

	// Bullish Spinning Top
	if isBullishSpinningTop(latest) {
		return "Bullish: Spinning Top"
	}

	// Bearish Spinning Top
	if isBearishSpinningTop(latest) {
		return "Bearish: Spinning Top"
	}

	return "?"
}

// detectChartPatterns analyzes the given price data to identify patterns
func detectChartPatterns(prices, highs, lows []float64) string {

	// Reversal Patterns
	if isHeadAndShoulders(prices) {
		return "Bearish: Head and Shoulders"
	}
	if isInverseHeadAndShoulders(prices) {
		return "Bullish: Head and Shoulders"
	}
	if isDoubleTop(prices) {
		return "Bearish: Double Top"
	}
	if isDoubleBottom(prices) {
		return "Bullish: Double Bottom"
	}
	if isVPattern(prices) {
		return "Bullish: V Pattern"
	}
	if isInvertedVPattern(prices) {
		return "Bearish: V Pattern"
	}
	if isRisingWedge(highs, lows) {
		return "Bearish: Rising Wedge"
	}
	if isFallingWedge(highs, lows) {
		return "Bullish: Falling Wedge"
	}

	// Continuation Patterns
	if isFlag(prices) {
		return "Continuation: Flag"
	}
	if isPennant(prices) {
		return "Continuation: Pennant"
	}
	if isRectangle(prices) {
		return "Continuation: Rectangle"
	}

	// Neutral Patterns
	if isSymmetricalTriangle(highs, lows) {
		return "Neutral: Symmetrical Triangle"
	}
	if isAscendingTriangle(highs, lows) {
		return "Neutral: Ascending Triangle"
	}
	if isDescendingTriangle(highs, lows) {
		return "Neutral: Descending Triangle"
	}

	return "?"
}

func OverallTrend(trend10, trend20, trend50, curPrice float64) string {

	if trend50 == 0 {
		if trend10 >= trend20 && curPrice >= trend10 {
			return Bullish
		}

		if trend10 <= trend20 && curPrice <= trend10 {
			return Bearish
		}
	}

	if trend20 == 0 {
		if curPrice >= trend10 {
			return Bullish
		}

		if curPrice <= trend10 {
			return Bearish
		}
	}

	if trend10 >= trend20 && trend20 >= trend50 && curPrice >= trend10 {
		return Bullish
	}

	if trend10 <= trend20 && trend20 <= trend50 && curPrice <= trend10 {
		return Bearish
	}

	return Neutral
}

/*
Updated Weighing System
Time Weight	Reason
1m		1		Minimally impactful to reduce distortion caused by noise.
3m		2		Still short-term but slightly more reliable than 1m.
5m		4		Core short-term time frame for intermediate decision-making.
15m		5		Key intermediate time frame, heavily weighted for scalping decisions.
30m		4		Provides confirmation of broader trends for intraday trading.
4h		3		Useful for context but less weighted due to your day trading style.
1d		2		Provides long-term perspective but less relevant for scalping.
*/

// TimeframetTrends
func TimeframeTrends(intervals map[string]Summary) string {
	trendName := ""
	maxScore := 0
	totalScore := 0
	timeWeights := map[string]int{
		"1m": 5, "5m": 10, "15m": 15, "30m": 20, "1h": 25, "4h": 30, "6h": 35, "12h": 40, "1d": 45, "3d": 50,
	}
	for timeframe, interval := range intervals {
		multiplier := timeWeights[timeframe]
		trendName = OverallTrend(interval.SMA10.Entry, interval.SMA20.Entry, interval.SMA50.Entry, interval.Candle.Close)
		trendScore := 0
		if trendName == Bullish {
			trendScore = 1 * multiplier
		} else if trendName == Bearish {
			trendScore = -1 * multiplier
		}
		totalScore += trendScore
		if interval.SMA10.Entry != 0 && interval.SMA20.Entry != 0 {
			maxScore += multiplier
		}
	}
	//Strong Bullish:
	minScoreStrongBullish := 0.7 * float64(maxScore)
	if float64(totalScore) >= minScoreStrongBullish {
		return "Strong Bullish"
	}

	//Bullish:
	minScoreBullish := 0.3 * float64(maxScore)
	if float64(totalScore) >= minScoreBullish {
		return "Bullish"
	}

	//Strong Bearish:
	minScoreStrongBearish := -0.7 * float64(maxScore)
	if float64(totalScore) <= minScoreStrongBearish {
		return "Strong Bearish"
	}

	//Bearish:
	minScoreBearish := -0.3 * float64(maxScore)
	if float64(totalScore) <= minScoreBearish {
		return "Bearish"
	}

	return Neutral
}

// tradingSummary creates the final analysis report.
func TradingSummary(pair, timeframe string, data MarketData) (Summary, error) {

	period10 := 10
	period20 := 20
	period50 := 50

	if len(data.Close) == 0 {
		return Summary{}, fmt.Errorf("Not market data provided for %s %s", pair, timeframe)
	}

	if len(data.Close) <= period50 {
		period50 = len(data.Close) - 1
	}
	if len(data.Close) <= period20 {
		period20 = len(data.Close) - 1
	}
	if len(data.Close) <= period10 {
		period10 = len(data.Close) - 1
	}

	var analysis10, analysis20, analysis50 trendAnalysis

	analysis20 = analyzeTrend(data, period20)
	analysis10 = analyzeTrend(data, period10)
	if len(data.Close) > period50 {
		analysis50 = analyzeTrend(data, period50)
	}

	rsiLength := 14
	if period20 < rsiLength {
		rsiLength = period20
	}
	smoothedRSI := CalculateSmoothedRSI(data.Close, rsiLength, 5)
	bollingerbands := calculateBollingerBands(data.Close, period20, 2)

	chartPattern := ""
	candlePattern := ""

	if len(data.Close) >= period10 {
		candleArray := []Candle{}
		lastClose := data.Close[len(data.Close)-period10:]
		lastOpen := data.Open[len(data.Open)-period10:]
		lastHigh := data.High[len(data.High)-period10:]
		lastLow := data.Low[len(data.Low)-period10:]

		for i := 0; i < len(lastClose)-1; i++ {
			candleArray = append(candleArray, Candle{
				Close: lastClose[i],
				Open:  lastOpen[i],
				High:  lastHigh[i],
				Low:   lastLow[i],
			})
		}
		chartPattern = detectChartPatterns(lastClose, lastHigh, lastLow)
		candlePattern = identifyCandlestickPattern(candleArray[:len(candleArray)-1])
	}

	var currentCandle Candle
	if len(data.Close) > 1 {
		currentCandle.Close = data.Close[len(data.Close)-1]
		currentCandle.High = data.High[len(data.High)-1]
		currentCandle.Low = data.Low[len(data.Low)-1]
		currentCandle.Open = data.Open[len(data.Open)-1]
	}

	trendName := OverallTrend(analysis10.Entry, analysis20.Entry, analysis50.Entry, currentCandle.Close)
	return Summary{
		Timeframe: timeframe,
		Trend:     trendName,
		Pattern: SummaryPattern{
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
			currentCandle.High,
			currentCandle.Low),
	}, nil
}
