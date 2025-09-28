package strategy

import (
	"math"

	"github.com/shopspring/decimal"
)

// SMA calculates the Simple Moving Average
func SMA(prices []decimal.Decimal, period int) decimal.Decimal {
	if len(prices) < period {
		return decimal.Zero
	}

	sum := decimal.Zero
	for i := len(prices) - period; i < len(prices); i++ {
		sum = sum.Add(prices[i])
	}

	return sum.Div(decimal.NewFromInt(int64(period)))
}

// RSI calculates the Relative Strength Index
func RSI(prices []decimal.Decimal, period int) float64 {
	if len(prices) < period+1 {
		return 50.0 // Neutral RSI
	}

	gains := decimal.Zero
	losses := decimal.Zero

	// Calculate initial average gain/loss
	for i := len(prices) - period; i < len(prices); i++ {
		change := prices[i].Sub(prices[i-1])
		if change.GreaterThan(decimal.Zero) {
			gains = gains.Add(change)
		} else {
			losses = losses.Add(change.Abs())
		}
	}

	avgGain := gains.Div(decimal.NewFromInt(int64(period)))
	avgLoss := losses.Div(decimal.NewFromInt(int64(period)))

	if avgLoss.IsZero() {
		return 100.0
	}

	rs := avgGain.Div(avgLoss)
	rsi := decimal.NewFromInt(100).Sub(
		decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(rs)),
	)

	rsiFloat, _ := rsi.Float64()
	return rsiFloat
}

// BollingerBands calculates Bollinger Bands (upper, middle, lower)
func BollingerBands(prices []decimal.Decimal, period int, stdDevMultiplier float64) (upper, middle, lower decimal.Decimal) {
	if len(prices) < period {
		return decimal.Zero, decimal.Zero, decimal.Zero
	}

	// Calculate middle band (SMA)
	middle = SMA(prices, period)

	// Calculate standard deviation
	recentPrices := prices[len(prices)-period:]
	variance := 0.0

	middleFloat, _ := middle.Float64()
	for _, price := range recentPrices {
		priceFloat, _ := price.Float64()
		diff := priceFloat - middleFloat
		variance += diff * diff
	}

	variance = variance / float64(period)
	stdDev := math.Sqrt(variance)

	// Calculate upper and lower bands
	stdDevDecimal := decimal.NewFromFloat(stdDev * stdDevMultiplier)
	upper = middle.Add(stdDevDecimal)
	lower = middle.Sub(stdDevDecimal)

	return upper, middle, lower
}

// EMA calculates the Exponential Moving Average
func EMA(prices []decimal.Decimal, period int) decimal.Decimal {
	if len(prices) < period {
		return decimal.Zero
	}

	// Start with SMA for the first value
	ema := SMA(prices[:period], period)

	// Calculate multiplier
	multiplier := decimal.NewFromFloat(2.0 / float64(period+1))

	// Calculate EMA for remaining prices
	for i := period; i < len(prices); i++ {
		ema = prices[i].Sub(ema).Mul(multiplier).Add(ema)
	}

	return ema
}

// MACD calculates the Moving Average Convergence Divergence
func MACD(prices []decimal.Decimal, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, histogram decimal.Decimal) {
	if len(prices) < slowPeriod {
		return decimal.Zero, decimal.Zero, decimal.Zero
	}

	// Calculate MACD line (12-period EMA - 26-period EMA)
	fastEMA := EMA(prices, fastPeriod)
	slowEMA := EMA(prices, slowPeriod)
	macd = fastEMA.Sub(slowEMA)

	// For signal line, we would need to calculate EMA of MACD values
	// Simplified version: return MACD only
	signal = decimal.Zero
	histogram = macd

	return macd, signal, histogram
}

// ATR calculates the Average True Range
func ATR(highs, lows, closes []decimal.Decimal, period int) decimal.Decimal {
	if len(highs) < period+1 || len(lows) < period+1 || len(closes) < period+1 {
		return decimal.Zero
	}

	trueRanges := make([]decimal.Decimal, 0, len(highs)-1)

	for i := 1; i < len(highs); i++ {
		// True Range = max(high-low, |high-prevClose|, |low-prevClose|)
		tr1 := highs[i].Sub(lows[i])
		tr2 := highs[i].Sub(closes[i-1]).Abs()
		tr3 := lows[i].Sub(closes[i-1]).Abs()

		tr := tr1
		if tr2.GreaterThan(tr) {
			tr = tr2
		}
		if tr3.GreaterThan(tr) {
			tr = tr3
		}

		trueRanges = append(trueRanges, tr)
	}

	// Calculate ATR as SMA of true ranges
	return SMA(trueRanges, period)
}

// StdDev calculates the standard deviation
func StdDev(prices []decimal.Decimal, period int) float64 {
	if len(prices) < period {
		return 0.0
	}

	recentPrices := prices[len(prices)-period:]
	mean := SMA(recentPrices, period)
	meanFloat, _ := mean.Float64()

	variance := 0.0
	for _, price := range recentPrices {
		priceFloat, _ := price.Float64()
		diff := priceFloat - meanFloat
		variance += diff * diff
	}

	variance = variance / float64(period)
	return math.Sqrt(variance)
}

