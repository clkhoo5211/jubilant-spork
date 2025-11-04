package main

import (
	"fmt"
	"log"
	"nofx/market"
)

func main() {
	fmt.Println("=" + string(make([]byte, 70)))
	fmt.Println("🔍 Alpaca Crypto Provider Integration Test")
	fmt.Println("=" + string(make([]byte, 70)))
	fmt.Println()

	// Initialize all providers (including Alpaca)
	market.InitializeProviders()

	// Test 1: Verify provider registration
	fmt.Println("📋 Test 1: Verify Provider Registration")
	fmt.Println("   " + string(make([]byte, 60)))
	
	providers := market.ListProviders()
	fmt.Printf("✅ Total registered providers: %d\n", len(providers))
	
	alpacaFound := false
	for _, name := range providers {
		if name == "alpaca_crypto" {
			alpacaFound = true
			fmt.Printf("✅ Alpaca provider found: %s\n", name)
		}
	}
	
	if !alpacaFound {
		log.Fatal("❌ Alpaca provider not found in registered providers!")
	}
	fmt.Println()

	// Test 2: Get Alpaca provider
	fmt.Println("📋 Test 2: Get Alpaca Provider Instance")
	fmt.Println("   " + string(make([]byte, 60)))
	
	alpacaProvider, err := market.GetProvider("alpaca_crypto")
	if err != nil {
		log.Fatalf("❌ Failed to get Alpaca provider: %v", err)
	}
	
	fmt.Printf("✅ Successfully retrieved provider: %s\n", alpacaProvider.GetName())
	fmt.Println()

	// Test 3: Symbol normalization
	fmt.Println("📋 Test 3: Symbol Normalization")
	fmt.Println("   " + string(make([]byte, 60)))
	
	testSymbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	for _, symbol := range testSymbols {
		normalized := alpacaProvider.NormalizeSymbol(symbol)
		fmt.Printf("   %s -> %s\n", symbol, normalized)
	}
	fmt.Println()

	// Test 4: Set as default and test GetKlines
	fmt.Println("📋 Test 4: Test GetKlines (Fetching Historical Data)")
	fmt.Println("   " + string(make([]byte, 60)))
	
	// Set Alpaca as default
	if err := market.SetDefaultProviderName("alpaca_crypto"); err != nil {
		log.Fatalf("❌ Failed to set Alpaca as default: %v", err)
	}
	fmt.Println("✅ Set Alpaca as default provider")
	
	// Test fetching klines for BTCUSDT
	fmt.Println("\n   Fetching 10 1-minute klines for BTCUSDT...")
	klines, err := alpacaProvider.GetKlines("BTCUSDT", "1m", 10)
	if err != nil {
		log.Printf("⚠️  GetKlines failed: %v", err)
		log.Printf("   This might be expected if Alpaca API is unavailable or requires authentication")
	} else {
		fmt.Printf("✅ Successfully fetched %d klines\n", len(klines))
		if len(klines) > 0 {
			latest := klines[len(klines)-1]
			fmt.Printf("   Latest kline: Open=%.2f, High=%.2f, Low=%.2f, Close=%.2f, Volume=%.2f\n",
				latest.Open, latest.High, latest.Low, latest.Close, latest.Volume)
		}
	}
	fmt.Println()

	// Test 5: Test with market.Get (using default provider)
	fmt.Println("📋 Test 5: Test Integration with market.Get()")
	fmt.Println("   " + string(make([]byte, 60)))
	
	fmt.Println("   Fetching market data for BTCUSDT using market.Get()...")
	data, err := market.Get("BTCUSDT")
	if err != nil {
		log.Printf("⚠️  market.Get failed: %v", err)
		log.Printf("   This might be expected if Alpaca API is unavailable")
	} else {
		fmt.Printf("✅ Successfully fetched market data\n")
		fmt.Printf("   Symbol: %s\n", data.Symbol)
		fmt.Printf("   Current Price: %.2f\n", data.CurrentPrice)
		fmt.Printf("   EMA20: %.2f\n", data.CurrentEMA20)
	}
	fmt.Println()

	// Test 6: Test OI and Funding Rate (expected to fail)
	fmt.Println("📋 Test 6: Test Open Interest & Funding Rate (Expected: Not Supported)")
	fmt.Println("   " + string(make([]byte, 60)))
	
	oi, err := alpacaProvider.GetOpenInterest("BTCUSDT")
	if err != nil {
		fmt.Printf("✅ GetOpenInterest correctly returns error: %v\n", err)
	} else {
		fmt.Printf("⚠️  GetOpenInterest returned: %+v\n", oi)
	}
	
	fr, err := alpacaProvider.GetFundingRate("BTCUSDT")
	if err != nil {
		fmt.Printf("✅ GetFundingRate correctly returns error: %v\n", err)
	} else {
		fmt.Printf("⚠️  GetFundingRate returned: %.6f\n", fr)
	}
	fmt.Println()

	fmt.Println("=" + string(make([]byte, 70)))
	fmt.Println("✅ Integration Test Completed!")
	fmt.Println("=" + string(make([]byte, 70)))
}

