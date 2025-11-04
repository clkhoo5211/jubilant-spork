package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

func main() {
	// Get API credentials from environment variables (optional for crypto data)
	apiKey := os.Getenv("ALPACA_API_KEY")
	apiSecret := os.Getenv("ALPACA_SECRET_KEY")

	fmt.Println("🔍 Comprehensive Alpaca Crypto Bars API Test")
	fmt.Println("📚 Reference: https://docs.alpaca.markets/reference/cryptolatestbars-1")
	fmt.Println()

	if apiKey == "" || apiSecret == "" {
		fmt.Println("⚠️  No API credentials provided (optional for crypto market data)")
		fmt.Println("ℹ️  Set ALPACA_API_KEY and ALPACA_SECRET_KEY for authenticated requests")
	} else {
		fmt.Println("✅ API credentials detected")
	}

	// Test symbols
	symbols := []string{"BTC/USD", "ETH/USD", "SOL/USD"}
	singleSymbol := "BTC/USD"

	fmt.Println("\n" + "=" + string(make([]byte, 70)))

	// ========================================================================
	// Test 1: Latest Bars (Multiple Symbols)
	// ========================================================================
	fmt.Println("\n📊 Test 1: Latest Bars (Multiple Symbols)")
	fmt.Println("   Endpoint: GET /v1beta3/crypto/{loc}/latest/bars")
	fmt.Println("   Function: GetLatestCryptoBars()")
	fmt.Println("   " + string(make([]byte, 60)))

	bars, err := marketdata.GetLatestCryptoBars(symbols, marketdata.GetLatestCryptoBarRequest{
		CryptoFeed: marketdata.US,
	})
	if err != nil {
		log.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Success! Retrieved latest bars for %d symbols\n", len(bars))
		for symbol, bar := range bars {
			fmt.Printf("   💰 %s: $%.2f (O:%.2f H:%.2f L:%.2f) @ %s\n",
				symbol, bar.Close, bar.Open, bar.High, bar.Low,
				bar.Timestamp.Format("15:04:05"))
		}
	}

	// ========================================================================
	// Test 2: Latest Bar (Single Symbol)
	// ========================================================================
	fmt.Println("\n📊 Test 2: Latest Bar (Single Symbol)")
	fmt.Println("   Endpoint: GET /v1beta3/crypto/{loc}/latest/bars/{symbol}")
	fmt.Println("   Function: GetLatestCryptoBar()")
	fmt.Println("   " + string(make([]byte, 60)))

	bar, err := marketdata.GetLatestCryptoBar(singleSymbol, marketdata.GetLatestCryptoBarRequest{
		CryptoFeed: marketdata.US,
	})
	if err != nil {
		log.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Success! Retrieved latest bar for %s\n", singleSymbol)
		fmt.Printf("   Timestamp: %s\n", bar.Timestamp.Format("2006-01-02 15:04:05 UTC"))
		fmt.Printf("   OHLC: O=%.2f H=%.2f L=%.2f C=%.2f\n", bar.Open, bar.High, bar.Low, bar.Close)
		fmt.Printf("   Volume: %.2f\n", bar.Volume)
		if bar.VWAP > 0 {
			fmt.Printf("   VWAP: %.2f\n", bar.VWAP)
		}
	}

	// ========================================================================
	// Test 3: Historical Bars (Single Symbol)
	// ========================================================================
	fmt.Println("\n📊 Test 3: Historical Bars (Single Symbol)")
	fmt.Println("   Endpoint: GET /v1beta3/crypto/{loc}/bars")
	fmt.Println("   Function: GetCryptoBars()")
	fmt.Println("   " + string(make([]byte, 60)))

	// Get last 20 bars (1-minute intervals)
	endTime := time.Now()
	startTime := endTime.Add(-20 * time.Minute) // Last 20 minutes

	fmt.Printf("   Timeframe: 1Min\n")
	fmt.Printf("   Start: %s\n", startTime.Format("15:04:05"))
	fmt.Printf("   End:   %s\n", endTime.Format("15:04:05"))
	fmt.Println()

	historicalBars, err := marketdata.GetCryptoBars(singleSymbol, marketdata.GetCryptoBarsRequest{
		TimeFrame:  marketdata.OneMin,
		Start:      startTime,
		End:        endTime,
		CryptoFeed: marketdata.US,
	})
	if err != nil {
		log.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Success! Retrieved %d historical bars for %s\n", len(historicalBars), singleSymbol)
		if len(historicalBars) > 0 {
			fmt.Println("\n   Sample bars (first 3 and last 3):")
			displayCount := 3
			if len(historicalBars) < displayCount {
				displayCount = len(historicalBars)
			}

			// First few bars
			for i := 0; i < displayCount && i < len(historicalBars); i++ {
				bar := historicalBars[i]
				fmt.Printf("   [%d] %s: C=%.2f V=%.2f\n",
					i+1, bar.Timestamp.Format("15:04:05"), bar.Close, bar.Volume)
			}

			if len(historicalBars) > displayCount*2 {
				fmt.Printf("   ... (showing first %d and last %d) ...\n", displayCount, displayCount)
			}

			// Last few bars
			startIdx := len(historicalBars) - displayCount
			if startIdx > displayCount {
				for i := startIdx; i < len(historicalBars); i++ {
					bar := historicalBars[i]
					fmt.Printf("   [%d] %s: C=%.2f V=%.2f\n",
						i+1, bar.Timestamp.Format("15:04:05"), bar.Close, bar.Volume)
				}
			}
		}
	}

	// ========================================================================
	// Test 4: Historical Bars (Multiple Symbols)
	// ========================================================================
	fmt.Println("\n📊 Test 4: Historical Bars (Multiple Symbols)")
	fmt.Println("   Endpoint: GET /v1beta3/crypto/{loc}/bars")
	fmt.Println("   Function: GetCryptoMultiBars()")
	fmt.Println("   " + string(make([]byte, 60)))

	// Get last 10 bars for multiple symbols
	multiEndTime := time.Now()
	multiStartTime := multiEndTime.Add(-10 * time.Minute) // Last 10 minutes

	fmt.Printf("   Symbols: %v\n", symbols)
	fmt.Printf("   Timeframe: 1Min\n")
	fmt.Printf("   Time range: Last 10 minutes\n")
	fmt.Println()

	multiBars, err := marketdata.GetCryptoMultiBars(symbols, marketdata.GetCryptoBarsRequest{
		TimeFrame:  marketdata.OneMin,
		Start:      multiStartTime,
		End:        multiEndTime,
		CryptoFeed: marketdata.US,
	})
	if err != nil {
		log.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Success! Retrieved historical bars for %d symbols\n", len(multiBars))
		for symbol, bars := range multiBars {
			fmt.Printf("   💰 %s: %d bars\n", symbol, len(bars))
			if len(bars) > 0 {
				latestBar := bars[len(bars)-1]
				fmt.Printf("      Latest: $%.2f @ %s\n",
					latestBar.Close, latestBar.Timestamp.Format("15:04:05"))
			}
		}
	}

	// ========================================================================
	// Test 5: Different Timeframes
	// ========================================================================
	fmt.Println("\n📊 Test 5: Historical Bars with Different Timeframes")
	fmt.Println("   Testing: 1Min, 5Min, 15Min, 1Hour")
	fmt.Println("   " + string(make([]byte, 60)))

	timeframes := []struct {
		name      string
		timeframe marketdata.TimeFrame
		minutes   int
	}{
		{"1Min", marketdata.OneMin, 60},
		{"5Min", marketdata.NewTimeFrame(5, marketdata.Min), 60},
		{"15Min", marketdata.NewTimeFrame(15, marketdata.Min), 60},
		{"1Hour", marketdata.NewTimeFrame(1, marketdata.Hour), 24},
	}

	for _, tf := range timeframes {
		start := time.Now().Add(-time.Duration(tf.minutes) * time.Minute)
		
		bars, err := marketdata.GetCryptoBars(singleSymbol, marketdata.GetCryptoBarsRequest{
			TimeFrame:  tf.timeframe,
			Start:      start,
			End:        time.Now(),
			TotalLimit: 10, // Limit to 10 bars for testing
			CryptoFeed: marketdata.US,
		})
		
		if err != nil {
			fmt.Printf("   ❌ %s: Error - %v\n", tf.name, err)
		} else {
			fmt.Printf("   ✅ %s: Retrieved %d bars\n", tf.name, len(bars))
			if len(bars) > 0 {
				latest := bars[len(bars)-1]
				fmt.Printf("      Latest: $%.2f @ %s\n",
					latest.Close, latest.Timestamp.Format("15:04:05"))
			}
		}
	}

	// ========================================================================
	// Summary
	// ========================================================================
	fmt.Println("\n" + "=" + string(make([]byte, 70)))
	fmt.Println("\n✅ All Tests Completed!")
	fmt.Println("\n📋 Test Summary:")
	fmt.Println("   ✓ Test 1: Latest Bars (Multiple Symbols)")
	fmt.Println("   ✓ Test 2: Latest Bar (Single Symbol)")
	fmt.Println("   ✓ Test 3: Historical Bars (Single Symbol)")
	fmt.Println("   ✓ Test 4: Historical Bars (Multiple Symbols)")
	fmt.Println("   ✓ Test 5: Different Timeframes (1Min, 5Min, 15Min, 1Hour)")
	fmt.Println("\n💡 API Endpoints Tested:")
	fmt.Println("   • GET /v1beta3/crypto/{loc}/latest/bars - Latest multi-bars")
	fmt.Println("   • GET /v1beta3/crypto/{loc}/latest/bars/{symbol} - Latest single bar")
	fmt.Println("   • GET /v1beta3/crypto/{loc}/bars - Historical bars")
	fmt.Println("\n📚 Documentation: https://docs.alpaca.markets/reference/cryptolatestbars-1")
}
