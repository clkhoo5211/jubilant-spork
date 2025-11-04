package main

import (
	"fmt"
	"nofx/market"
)

func main() {
	market.InitializeProviders()
	
	// Get provider
	p, err := market.GetProvider("alpaca_crypto")
	if err != nil {
		fmt.Printf("❌ Error getting provider: %v\n", err)
		return
	}
	fmt.Printf("✅ Provider: %s\n", p.GetName())
	
	// Test symbol normalization
	fmt.Printf("✅ Symbol normalization: BTCUSDT -> %s\n", p.NormalizeSymbol("BTCUSDT"))
	
	// Test GetKlines
	klines, err := p.GetKlines("BTCUSDT", "1m", 5)
	if err != nil {
		fmt.Printf("❌ GetKlines error: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Success: Got %d klines\n", len(klines))
	if len(klines) > 0 {
		latest := klines[len(klines)-1]
		fmt.Printf("✅ Latest kline: Close=%.2f, Volume=%.2f\n", latest.Close, latest.Volume)
	}
}

