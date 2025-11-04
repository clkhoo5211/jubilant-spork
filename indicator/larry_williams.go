package indicator

import (
	"fmt"
	"math"
	"nofx/market"
)

// LarryWilliamsSignal represents the signal direction
type LarryWilliamsSignal string

const (
	LarryWilliamsLONG  LarryWilliamsSignal = "LONG"
	LarryWilliamsSHORT LarryWilliamsSignal = "SHORT"
	LarryWilliamsWAIT  LarryWilliamsSignal = "WAIT"
)

// LarryWilliamsResult contains the analysis result
type LarryWilliamsResult struct {
	SignalType LarryWilliamsSignal
	Confidence float64
	Strength   float64
	BodyRatio  float64
	Reasoning  []string
}

// DetectLarryWilliams detects Larry Williams Outside Bar patterns (simplified version)
// Strategy: Contrarian logic - Outside Bar with body ratio filter
// Signal: Close < Previous Low → LONG, Close > Previous High → SHORT
func DetectLarryWilliams(klines []market.Kline, atr14 float64) LarryWilliamsResult {
	if len(klines) < 2 {
		return LarryWilliamsResult{
			SignalType: LarryWilliamsWAIT,
			Confidence: 0.0,
			Strength:   0.0,
			Reasoning:  []string{"Insufficient data for Larry Williams analysis"},
		}
	}
	
	current := klines[len(klines)-1]
	previous := klines[len(klines)-2]
	
	// Check if current bar is an outside bar
	isOutsideBar := current.High > previous.High && current.Low < previous.Low
	
	if !isOutsideBar {
		return LarryWilliamsResult{
			SignalType: LarryWilliamsWAIT,
			Confidence: 0.0,
			Strength:   0.0,
			Reasoning:  []string{"No outside bar pattern detected"},
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
		return LarryWilliamsResult{
			SignalType: LarryWilliamsWAIT,
			BodyRatio:  bodyRatio,
			Confidence: 0.0,
			Strength:   0.0,
			Reasoning:  []string{fmt.Sprintf("Body ratio %.2f below minimum %.2f", bodyRatio, minRatio)},
		}
	}
	
	// Determine signal direction (contrarian logic)
	// Close < Previous Low → LONG (expecting reversal up)
	// Close > Previous High → SHORT (expecting reversal down)
	var signalType LarryWilliamsSignal
	var confidence, strength float64
	
	if current.Close < previous.Low {
		signalType = LarryWilliamsLONG
		confidence = 0.75
		strength = math.Min(bodyRatio/3.0, 1.0) // Strength increases with body ratio
		return LarryWilliamsResult{
			SignalType: signalType,
			Confidence: confidence,
			Strength:   strength,
			BodyRatio:  bodyRatio,
			Reasoning: []string{
				"Larry Williams Outside Bar pattern detected",
				fmt.Sprintf("Close (%.2f) < Previous Low (%.2f) → LONG signal (contrarian)", current.Close, previous.Low),
				fmt.Sprintf("Body ratio: %.2f", bodyRatio),
				"Contrarian logic: negative signal → positive move expected",
			},
		}
	} else if current.Close > previous.High {
		signalType = LarryWilliamsSHORT
		confidence = 0.75
		strength = math.Min(bodyRatio/3.0, 1.0)
		return LarryWilliamsResult{
			SignalType: signalType,
			Confidence: confidence,
			Strength:   strength,
			BodyRatio:  bodyRatio,
			Reasoning: []string{
				"Larry Williams Outside Bar pattern detected",
				fmt.Sprintf("Close (%.2f) > Previous High (%.2f) → SHORT signal (contrarian)", current.Close, previous.High),
				fmt.Sprintf("Body ratio: %.2f", bodyRatio),
				"Contrarian logic: negative signal → positive move expected",
			},
		}
	}
	
	// Neither condition met
	return LarryWilliamsResult{
		SignalType: LarryWilliamsWAIT,
		BodyRatio:  bodyRatio,
		Confidence: 0.0,
		Strength:   0.0,
		Reasoning:  []string{"Outside bar pattern detected but no clear signal direction"},
	}
}

