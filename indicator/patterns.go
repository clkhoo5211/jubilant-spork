package indicator

import (
	"math"
	"nofx/market"
)

// CandleProperties holds calculated properties of a single candle
type CandleProperties struct {
	Range               float64
	BodySize            float64
	BodyPercent         float64
	IsBullish           bool
	IsBearish           bool
	UpperShadow         float64
	LowerShadow         float64
	UpperShadowPercent  float64
	LowerShadowPercent  float64
	UpperShadowRangePercent float64
	LowerShadowRangePercent float64
}

// PatternResult represents a detected pattern
type PatternResult struct {
	Pattern string
	IsBullish bool
	Confidence float64 // 0.0 to 1.0
}

// calculateCandleProperties calculates basic properties for a candle
func calculateCandleProperties(kline market.Kline) CandleProperties {
	candleRange := kline.High - kline.Low
	bodySize := math.Abs(kline.Close - kline.Open)
	
	var bodyPercent float64
	if candleRange > 0 {
		bodyPercent = (bodySize / candleRange) * 100
	}
	
	isBullish := kline.Close > kline.Open
	isBearish := kline.Close < kline.Open
	
	var upperShadow, lowerShadow float64
	if isBullish {
		upperShadow = kline.High - kline.Close
		lowerShadow = kline.Open - kline.Low
	} else {
		upperShadow = kline.High - kline.Open
		lowerShadow = kline.Close - kline.Low
	}
	
	safeBodySize := bodySize
	if safeBodySize == 0 {
		safeBodySize = 1e-6
	}
	
	upperShadowPercent := (upperShadow / safeBodySize) * 100
	lowerShadowPercent := (lowerShadow / safeBodySize) * 100
	
	var upperShadowRangePercent, lowerShadowRangePercent float64
	if candleRange > 0 {
		upperShadowRangePercent = (upperShadow / candleRange) * 100
		lowerShadowRangePercent = (lowerShadow / candleRange) * 100
	}
	
	return CandleProperties{
		Range:                  candleRange,
		BodySize:               bodySize,
		BodyPercent:            bodyPercent,
		IsBullish:              isBullish,
		IsBearish:              isBearish,
		UpperShadow:            upperShadow,
		LowerShadow:            lowerShadow,
		UpperShadowPercent:     upperShadowPercent,
		LowerShadowPercent:     lowerShadowPercent,
		UpperShadowRangePercent: upperShadowRangePercent,
		LowerShadowRangePercent: lowerShadowRangePercent,
	}
}

// DetectCandlestickPatterns detects all candlestick patterns on the latest candles
func DetectCandlestickPatterns(klines []market.Kline) []PatternResult {
	if len(klines) < 3 {
		return []PatternResult{}
	}
	
	var results []PatternResult
	dojiBodyPercent := 5.0 // Threshold for doji patterns
	
	// Get latest candles
	current := klines[len(klines)-1]
	previous := klines[len(klines)-2]
	var previous2 market.Kline
	if len(klines) >= 3 {
		previous2 = klines[len(klines)-3]
	}
	
	currentProps := calculateCandleProperties(current)
	previousProps := calculateCandleProperties(previous)
	previous2Props := calculateCandleProperties(previous2)
	
	// SINGLE CANDLE PATTERNS
	
	// 1. Hammer / Hanging Man
	if hammer := detectHammer(current, currentProps, klines); hammer.Pattern != "" {
		results = append(results, hammer)
	}
	
	// 2. Inverted Hammer / Shooting Star
	if inverted := detectInvertedHammer(current, currentProps, klines); inverted.Pattern != "" {
		results = append(results, inverted)
	}
	
	// 3. Dragonfly Doji
	if df := detectDragonflyDoji(currentProps, dojiBodyPercent); df.Pattern != "" {
		results = append(results, df)
	}
	
	// 4. Gravestone Doji
	if gs := detectGravestoneDoji(currentProps, dojiBodyPercent); gs.Pattern != "" {
		results = append(results, gs)
	}
	
	// 5. Standard Doji
	if doji := detectDoji(currentProps, dojiBodyPercent); doji.Pattern != "" {
		// Check if it's not already a specialized doji
		isDragonfly := detectDragonflyDoji(currentProps, dojiBodyPercent).Pattern != ""
		isGravestone := detectGravestoneDoji(currentProps, dojiBodyPercent).Pattern != ""
		if !isDragonfly && !isGravestone {
			results = append(results, doji)
		}
	}
	
	// 6. Marubozu
	if marubozu := detectMarubozu(currentProps); marubozu.Pattern != "" {
		results = append(results, marubozu)
	}
	
	// 7. Spinning Top
	if spinning := detectSpinningTop(currentProps); spinning.Pattern != "" {
		results = append(results, spinning)
	}
	
	// TWO CANDLE PATTERNS
	
	// 8. Bullish Engulfing
	if engulf := detectBullishEngulfing(current, previous, currentProps, previousProps); engulf.Pattern != "" {
		results = append(results, engulf)
	}
	
	// 9. Bearish Engulfing
	if engulf := detectBearishEngulfing(current, previous, currentProps, previousProps); engulf.Pattern != "" {
		results = append(results, engulf)
	}
	
	// 10. Bullish Harami
	if harami := detectBullishHarami(current, previous, currentProps, previousProps); harami.Pattern != "" {
		results = append(results, harami)
	}
	
	// 11. Bearish Harami
	if harami := detectBearishHarami(current, previous, currentProps, previousProps); harami.Pattern != "" {
		results = append(results, harami)
	}
	
	// 12. Tweezer Top
	if tweezer := detectTweezerTop(current, previous, klines); tweezer.Pattern != "" {
		results = append(results, tweezer)
	}
	
	// 13. Tweezer Bottom
	if tweezer := detectTweezerBottom(current, previous, klines); tweezer.Pattern != "" {
		results = append(results, tweezer)
	}
	
	// THREE CANDLE PATTERNS (need previous2)
	if len(klines) >= 3 {
		// 14. Morning Star
		if morning := detectMorningStar(current, previous, previous2, currentProps, previousProps, previous2Props); morning.Pattern != "" {
			results = append(results, morning)
		}
		
		// 15. Evening Star
		if evening := detectEveningStar(current, previous, previous2, currentProps, previousProps, previous2Props); evening.Pattern != "" {
			results = append(results, evening)
		}
		
		// 16. Three White Soldiers
		if soldiers := detectThreeWhiteSoldiers(current, previous, previous2, currentProps, previousProps, previous2Props); soldiers.Pattern != "" {
			results = append(results, soldiers)
		}
		
		// 17. Three Black Crows
		if crows := detectThreeBlackCrows(current, previous, previous2, currentProps, previousProps, previous2Props); crows.Pattern != "" {
			results = append(results, crows)
		}
		
		// 18. Abandoned Baby Bullish
		if baby := detectAbandonedBabyBullish(current, previous, previous2, currentProps, previousProps, previous2Props); baby.Pattern != "" {
			results = append(results, baby)
		}
		
		// 19. Abandoned Baby Bearish
		if baby := detectAbandonedBabyBearish(current, previous, previous2, currentProps, previousProps, previous2Props); baby.Pattern != "" {
			results = append(results, baby)
		}
	}
	
	return results
}

// Single candle patterns

func detectHammer(kline market.Kline, props CandleProperties, klines []market.Kline) PatternResult {
	conditions := props.BodyPercent < 30 &&
		props.LowerShadowPercent > 200 &&
		props.UpperShadowPercent < 20 &&
		props.LowerShadowRangePercent > 60
	
	if !conditions {
		return PatternResult{}
	}
	
	// Check trend for hanging man vs hammer
	isUptrend := false
	if len(klines) >= 6 {
		price5Ago := klines[len(klines)-6].Close
		isUptrend = kline.Close > price5Ago
	}
	
	if isUptrend {
		return PatternResult{
			Pattern:    "Hanging Man",
			IsBullish:  false,
			Confidence: 0.7,
		}
	}
	
	return PatternResult{
		Pattern:    "Hammer",
		IsBullish:  true,
		Confidence: 0.7,
	}
}

func detectInvertedHammer(kline market.Kline, props CandleProperties, klines []market.Kline) PatternResult {
	conditions := props.BodyPercent < 30 &&
		props.UpperShadowPercent > 200 &&
		props.LowerShadowPercent < 20 &&
		props.UpperShadowRangePercent > 60
	
	if !conditions {
		return PatternResult{}
	}
	
	// Check trend for shooting star vs inverted hammer
	isUptrend := false
	if len(klines) >= 6 {
		price5Ago := klines[len(klines)-6].Close
		isUptrend = kline.Close > price5Ago
	}
	
	if isUptrend {
		return PatternResult{
			Pattern:    "Shooting Star",
			IsBullish:  false,
			Confidence: 0.7,
		}
	}
	
	return PatternResult{
		Pattern:    "Inverted Hammer",
		IsBullish:  true,
		Confidence: 0.7,
	}
}

func detectDragonflyDoji(props CandleProperties, dojiBodyPercent float64) PatternResult {
	conditions := props.BodyPercent <= dojiBodyPercent &&
		props.LowerShadowRangePercent > 60 &&
		props.UpperShadowRangePercent < 10
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Dragonfly Doji",
		IsBullish:  true,
		Confidence: 0.75,
	}
}

func detectGravestoneDoji(props CandleProperties, dojiBodyPercent float64) PatternResult {
	conditions := props.BodyPercent <= dojiBodyPercent &&
		props.UpperShadowRangePercent > 60 &&
		props.LowerShadowRangePercent < 10
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Gravestone Doji",
		IsBullish:  false,
		Confidence: 0.75,
	}
}

func detectDoji(props CandleProperties, dojiBodyPercent float64) PatternResult {
	if props.BodyPercent > dojiBodyPercent {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Doji",
		IsBullish:  false, // Neutral
		Confidence: 0.6,
	}
}

func detectMarubozu(props CandleProperties) PatternResult {
	conditions := props.BodyPercent > 90 &&
		props.UpperShadowRangePercent < 5 &&
		props.LowerShadowRangePercent < 5
	
	if !conditions {
		return PatternResult{}
	}
	
	isBullish := props.IsBullish
	return PatternResult{
		Pattern:    "Marubozu",
		IsBullish:  isBullish,
		Confidence: 0.8,
	}
}

func detectSpinningTop(props CandleProperties) PatternResult {
	conditions := props.BodyPercent < 25 &&
		props.UpperShadowRangePercent > 25 &&
		props.LowerShadowRangePercent > 25
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Spinning Top",
		IsBullish:  false, // Neutral/reversal
		Confidence: 0.6,
	}
}

// Two candle patterns

func detectBullishEngulfing(current, previous market.Kline, currentProps, previousProps CandleProperties) PatternResult {
	conditions := currentProps.IsBullish &&
		previousProps.IsBearish &&
		current.Open < previous.Close &&
		current.Close > previous.Open &&
		currentProps.BodyPercent > 50
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Bullish Engulfing",
		IsBullish:  true,
		Confidence: 0.8,
	}
}

func detectBearishEngulfing(current, previous market.Kline, currentProps, previousProps CandleProperties) PatternResult {
	conditions := currentProps.IsBearish &&
		previousProps.IsBullish &&
		current.Open > previous.Close &&
		current.Close < previous.Open &&
		currentProps.BodyPercent > 50
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Bearish Engulfing",
		IsBullish:  false,
		Confidence: 0.8,
	}
}

func detectBullishHarami(current, previous market.Kline, currentProps, previousProps CandleProperties) PatternResult {
	conditions := currentProps.IsBullish &&
		previousProps.IsBearish &&
		previousProps.BodySize > currentProps.BodySize &&
		current.Open > previous.Close &&
		current.Close < previous.Open
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Bullish Harami",
		IsBullish:  true,
		Confidence: 0.7,
	}
}

func detectBearishHarami(current, previous market.Kline, currentProps, previousProps CandleProperties) PatternResult {
	conditions := currentProps.IsBearish &&
		previousProps.IsBullish &&
		previousProps.BodySize > currentProps.BodySize &&
		current.Open < previous.Close &&
		current.Close > previous.Open
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Bearish Harami",
		IsBullish:  false,
		Confidence: 0.7,
	}
}

func detectTweezerTop(current, previous market.Kline, klines []market.Kline) PatternResult {
	// Highs should be nearly equal (within 0.1%)
	tolerance := previous.High * 0.001
	highsMatch := math.Abs(current.High - previous.High) < tolerance
	
	// Check trend
	isUptrend := false
	if len(klines) >= 4 {
		price3Ago := klines[len(klines)-4].Close
		isUptrend = previous.Close > price3Ago
	}
	
	// Previous was bullish, current is bearish
	previousBullish := previous.Close > previous.Open
	currentBearish := current.Close < current.Open
	
	conditions := previousBullish &&
		currentBearish &&
		highsMatch &&
		isUptrend
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Tweezer Top",
		IsBullish:  false,
		Confidence: 0.75,
	}
}

func detectTweezerBottom(current, previous market.Kline, klines []market.Kline) PatternResult {
	// Lows should be nearly equal (within 0.1%)
	tolerance := previous.Low * 0.001
	lowsMatch := math.Abs(current.Low - previous.Low) < tolerance
	
	// Check trend
	isDowntrend := false
	if len(klines) >= 4 {
		price3Ago := klines[len(klines)-4].Close
		isDowntrend = previous.Close < price3Ago
	}
	
	conditions := previous.Close < previous.Open && // Previous was bearish
		current.Close > current.Open && // Current is bullish
		lowsMatch &&
		isDowntrend
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Tweezer Bottom",
		IsBullish:  true,
		Confidence: 0.75,
	}
}

// Three candle patterns

func detectMorningStar(current, previous, previous2 market.Kline, currentProps, previousProps, previous2Props CandleProperties) PatternResult {
	conditions := previous2Props.IsBearish &&
		previousProps.BodyPercent < 30 &&
		currentProps.IsBullish &&
		current.Close > previous2.Open-(previous2Props.BodySize/2)
	
	// Check downtrend
	isDowntrend := previous2.Close < previous2.Open
	
	if !conditions || !isDowntrend {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Morning Star",
		IsBullish:  true,
		Confidence: 0.85,
	}
}

func detectEveningStar(current, previous, previous2 market.Kline, currentProps, previousProps, previous2Props CandleProperties) PatternResult {
	conditions := previous2Props.IsBullish &&
		previousProps.BodyPercent < 30 &&
		currentProps.IsBearish &&
		current.Close < previous2.Open+(previous2Props.BodySize/2)
	
	// Check uptrend
	isUptrend := previous2.Close > previous2.Open
	
	if !conditions || !isUptrend {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Evening Star",
		IsBullish:  false,
		Confidence: 0.85,
	}
}

func detectThreeWhiteSoldiers(current, previous, previous2 market.Kline, currentProps, previousProps, previous2Props CandleProperties) PatternResult {
	conditions := currentProps.IsBullish &&
		previousProps.IsBullish &&
		previous2Props.IsBullish &&
		current.Close > previous.Close &&
		previous.Close > previous2.Close &&
		currentProps.BodyPercent > 50 &&
		previousProps.BodyPercent > 50 &&
		previous2Props.BodyPercent > 50
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Three White Soldiers",
		IsBullish:  true,
		Confidence: 0.9,
	}
}

func detectThreeBlackCrows(current, previous, previous2 market.Kline, currentProps, previousProps, previous2Props CandleProperties) PatternResult {
	conditions := currentProps.IsBearish &&
		previousProps.IsBearish &&
		previous2Props.IsBearish &&
		current.Close < previous.Close &&
		previous.Close < previous2.Close &&
		currentProps.BodyPercent > 50 &&
		previousProps.BodyPercent > 50 &&
		previous2Props.BodyPercent > 50
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Three Black Crows",
		IsBullish:  false,
		Confidence: 0.9,
	}
}

func detectAbandonedBabyBullish(current, previous, previous2 market.Kline, currentProps, previousProps, previous2Props CandleProperties) PatternResult {
	// Previous candle should be a doji with gaps on both sides
	isDoji := previousProps.BodyPercent < 5
	hasGapDown := previous.Low > previous2.Close
	hasGapUp := previous.High < current.Close
	
	conditions := previous2Props.IsBearish &&
		isDoji &&
		hasGapDown &&
		hasGapUp &&
		currentProps.IsBullish
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Abandoned Baby Bullish",
		IsBullish:  true,
		Confidence: 0.85,
	}
}

func detectAbandonedBabyBearish(current, previous, previous2 market.Kline, currentProps, previousProps, previous2Props CandleProperties) PatternResult {
	// Previous candle should be a doji with gaps on both sides
	isDoji := previousProps.BodyPercent < 5
	hasGapUp := previous.High < previous2.Close
	hasGapDown := previous.Low > current.Close
	
	conditions := previous2Props.IsBullish &&
		isDoji &&
		hasGapUp &&
		hasGapDown &&
		currentProps.IsBearish
	
	if !conditions {
		return PatternResult{}
	}
	
	return PatternResult{
		Pattern:    "Abandoned Baby Bearish",
		IsBullish:  false,
		Confidence: 0.85,
	}
}

