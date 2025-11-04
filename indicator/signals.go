package indicator

import (
	"fmt"
	"strings"
)

// SignalSummary provides a human-readable summary of all detected signals
type SignalSummary struct {
	CandlestickPatterns []PatternResult
	OutsideDay          OutsideDayResult
	LarryWilliams       LarryWilliamsResult
}

// FormatAnalysis formats the analysis results into a readable string for AI prompts
func FormatAnalysis(summary SignalSummary) string {
	var parts []string
	
	// Candlestick patterns
	if len(summary.CandlestickPatterns) > 0 {
		parts = append(parts, "=== CANDLESTICK PATTERNS ===")
		bullishCount := 0
		bearishCount := 0
		
		for _, pattern := range summary.CandlestickPatterns {
			direction := "BEARISH"
			if pattern.IsBullish {
				direction = "BULLISH"
				bullishCount++
			} else if pattern.Pattern != "Doji" && pattern.Pattern != "Spinning Top" {
				bearishCount++
			}
			
			parts = append(parts, fmt.Sprintf("- %s (%s, Confidence: %.1f%%)", 
				pattern.Pattern, direction, pattern.Confidence*100))
		}
		
		parts = append(parts, fmt.Sprintf("Summary: %d bullish patterns, %d bearish patterns detected", 
			bullishCount, bearishCount))
		parts = append(parts, "")
	}
	
	// Outside Day
	if summary.OutsideDay.SignalType != OutsideDayWAIT {
		parts = append(parts, "=== OUTSIDE DAY PATTERN ===")
		parts = append(parts, fmt.Sprintf("Signal: %s", summary.OutsideDay.SignalType))
		parts = append(parts, fmt.Sprintf("Confidence: %.1f%%, Strength: %.1f%%", 
			summary.OutsideDay.Confidence*100, summary.OutsideDay.Strength*100))
		for _, reason := range summary.OutsideDay.Reasoning {
			parts = append(parts, fmt.Sprintf("  - %s", reason))
		}
		parts = append(parts, "")
	}
	
	// Larry Williams
	if summary.LarryWilliams.SignalType != LarryWilliamsWAIT {
		parts = append(parts, "=== LARRY WILLIAMS OUTSIDE BAR ===")
		parts = append(parts, fmt.Sprintf("Signal: %s", summary.LarryWilliams.SignalType))
		parts = append(parts, fmt.Sprintf("Confidence: %.1f%%, Strength: %.1f%%, Body Ratio: %.2f", 
			summary.LarryWilliams.Confidence*100, summary.LarryWilliams.Strength*100, summary.LarryWilliams.BodyRatio))
		for _, reason := range summary.LarryWilliams.Reasoning {
			parts = append(parts, fmt.Sprintf("  - %s", reason))
		}
		parts = append(parts, "")
	}
	
	// Overall signal summary
	if len(summary.CandlestickPatterns) > 0 || 
		summary.OutsideDay.SignalType != OutsideDayWAIT || 
		summary.LarryWilliams.SignalType != LarryWilliamsWAIT {
		parts = append(parts, "=== SIGNAL INTERPRETATION ===")
		
		// Count bullish vs bearish signals
		bullishSignals := 0
		bearishSignals := 0
		
		for _, p := range summary.CandlestickPatterns {
			if p.IsBullish {
				bullishSignals++
			} else if p.Pattern != "Doji" && p.Pattern != "Spinning Top" {
				bearishSignals++
			}
		}
		
		if summary.OutsideDay.SignalType == OutsideDayLONG {
			bullishSignals++
		} else if summary.OutsideDay.SignalType == OutsideDaySHORT {
			bearishSignals++
		}
		
		if summary.LarryWilliams.SignalType == LarryWilliamsLONG {
			bullishSignals++
		} else if summary.LarryWilliams.SignalType == LarryWilliamsSHORT {
			bearishSignals++
		}
		
		if bullishSignals > bearishSignals {
			parts = append(parts, fmt.Sprintf("Overall Bias: BULLISH (%d bullish vs %d bearish signals)", 
				bullishSignals, bearishSignals))
		} else if bearishSignals > bullishSignals {
			parts = append(parts, fmt.Sprintf("Overall Bias: BEARISH (%d bearish vs %d bullish signals)", 
				bearishSignals, bullishSignals))
		} else {
			parts = append(parts, fmt.Sprintf("Overall Bias: NEUTRAL (%d bullish, %d bearish)", 
				bullishSignals, bearishSignals))
		}
	}
	
	if len(parts) == 0 {
		return "No significant patterns detected in recent price action."
	}
	
	return strings.Join(parts, "\n")
}

