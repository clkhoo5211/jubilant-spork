package indicator

import (
	"nofx/market"
)

// Analyze performs comprehensive pattern analysis on market data
// Returns formatted string ready for AI prompt inclusion
func Analyze(marketData *market.Data) string {
	var summary SignalSummary
	
	// Get klines from intraday series (3m) for candlestick patterns
	// We need to reconstruct klines from market data
	// For now, we'll use a simplified approach - get klines directly
	var klines3m []market.Kline
	var klines4h []market.Kline
	
	// Try to get klines from provider
	provider, err := market.GetDefaultProvider()
	if err == nil && marketData != nil {
		// Get recent klines for pattern detection
		klines3m, _ = provider.GetKlines(marketData.Symbol, "3m", 40)
		klines4h, _ = provider.GetKlines(marketData.Symbol, "4h", 60)
	}
	
	// Detect candlestick patterns on 3m timeframe
	if len(klines3m) >= 3 {
		summary.CandlestickPatterns = DetectCandlestickPatterns(klines3m)
	}
	
	// Detect Outside Day on 4h timeframe
	if len(klines4h) >= 2 {
		summary.OutsideDay = DetectOutsideDay(klines4h)
	}
	
	// Detect Larry Williams on 4h timeframe
	if len(klines4h) >= 2 {
		atr14 := marketData.LongerTermContext.ATR14
		summary.LarryWilliams = DetectLarryWilliams(klines4h, atr14)
	}
	
	// Format and return analysis
	return FormatAnalysis(summary)
}

// AnalyzeWithKlines allows direct klines input (for testing or custom scenarios)
func AnalyzeWithKlines(symbol string, klines3m, klines4h []market.Kline, atr14 float64) string {
	var summary SignalSummary
	
	// Detect candlestick patterns
	if len(klines3m) >= 3 {
		summary.CandlestickPatterns = DetectCandlestickPatterns(klines3m)
	}
	
	// Detect Outside Day
	if len(klines4h) >= 2 {
		summary.OutsideDay = DetectOutsideDay(klines4h)
	}
	
	// Detect Larry Williams
	if len(klines4h) >= 2 {
		summary.LarryWilliams = DetectLarryWilliams(klines4h, atr14)
	}
	
	return FormatAnalysis(summary)
}

