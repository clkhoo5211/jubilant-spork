package indicator

import (
	"fmt"
	"math"
	"nofx/market"
)

// OutsideDaySignal represents the signal direction
type OutsideDaySignal string

const (
	OutsideDayLONG OutsideDaySignal = "LONG"
	OutsideDaySHORT OutsideDaySignal = "SHORT"
	OutsideDayWAIT OutsideDaySignal = "WAIT"
)

// OutsideDayResult contains the analysis result
type OutsideDayResult struct {
	SignalType OutsideDaySignal
	Confidence float64
	Strength   float64
	Reasoning  []string
}

// DetectOutsideDay detects Outside Day patterns (simplified version)
// Outside Day: Current high > Previous high AND Current low < Previous low
// Signal: Close < Previous Low → LONG, Close > Previous High → SHORT
func DetectOutsideDay(klines []market.Kline) OutsideDayResult {
	if len(klines) < 2 {
		return OutsideDayResult{
			SignalType: OutsideDayWAIT,
			Confidence: 0.0,
			Strength:   0.0,
			Reasoning:  []string{"Insufficient data for Outside Day analysis"},
		}
	}
	
	current := klines[len(klines)-1]
	previous := klines[len(klines)-2]
	
	// Check if current bar is an outside bar
	isOutsideBar := current.High > previous.High && current.Low < previous.Low
	
	if !isOutsideBar {
		return OutsideDayResult{
			SignalType: OutsideDayWAIT,
			Confidence: 0.0,
			Strength:   0.0,
			Reasoning:  []string{"No Outside Day pattern detected"},
		}
	}
	
	// Calculate body ratio (current body / previous body)
	currentBodySize := math.Abs(current.Close - current.Open)
	previousBodySize := math.Abs(previous.Close - previous.Open)
	
	var bodyRatio float64
	if previousBodySize > 0 {
		bodyRatio = currentBodySize / previousBodySize
	} else {
		bodyRatio = 0
	}
	
	// Minimum ratio filter (default 2.0)
	minRatio := 2.0
	if bodyRatio < minRatio {
		return OutsideDayResult{
			SignalType: OutsideDayWAIT,
			Confidence: 0.0,
			Strength:   0.0,
			Reasoning:  []string{fmt.Sprintf("Body ratio %.2f below minimum %.2f", bodyRatio, minRatio)},
		}
	}
	
	// Determine signal direction (contrarian logic)
	// Close < Previous Low → LONG (expecting reversal up)
	// Close > Previous High → SHORT (expecting reversal down)
	var signalType OutsideDaySignal
	var confidence, strength float64
	
	if current.Close < previous.Low {
		signalType = OutsideDayLONG
		confidence = 0.75
		strength = math.Min(bodyRatio/3.0, 1.0) // Strength increases with body ratio
		return OutsideDayResult{
			SignalType: signalType,
			Confidence: confidence,
			Strength:   strength,
			Reasoning: []string{
				"Outside Day pattern detected",
				fmt.Sprintf("Close (%.2f) < Previous Low (%.2f) → LONG signal", current.Close, previous.Low),
				fmt.Sprintf("Body ratio: %.2f", bodyRatio),
			},
		}
	} else if current.Close > previous.High {
		signalType = OutsideDaySHORT
		confidence = 0.75
		strength = math.Min(bodyRatio/3.0, 1.0)
		return OutsideDayResult{
			SignalType: signalType,
			Confidence: confidence,
			Strength:   strength,
			Reasoning: []string{
				"Outside Day pattern detected",
				fmt.Sprintf("Close (%.2f) > Previous High (%.2f) → SHORT signal", current.Close, previous.High),
				fmt.Sprintf("Body ratio: %.2f", bodyRatio),
			},
		}
	}
	
	// Neither condition met
	return OutsideDayResult{
		SignalType: OutsideDayWAIT,
		Confidence: 0.0,
		Strength:   0.0,
		Reasoning:  []string{"Outside Day pattern detected but no clear signal direction"},
	}
}

