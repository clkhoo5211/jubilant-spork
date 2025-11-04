package market

import (
	"fmt"
	"sync"
)

// MarketDataProvider defines the interface for fetching market data from different exchanges
type MarketDataProvider interface {
	// GetKlines fetches candlestick data
	GetKlines(symbol, interval string, limit int) ([]Kline, error)

	// GetOpenInterest fetches open interest data
	GetOpenInterest(symbol string) (*OIData, error)

	// GetFundingRate fetches funding rate
	GetFundingRate(symbol string) (float64, error)

	// NormalizeSymbol converts symbol to exchange format
	NormalizeSymbol(symbol string) string

	// GetName returns provider name
	GetName() string
}

// ProviderRegistry manages available market data providers
type ProviderRegistry struct {
	providers map[string]MarketDataProvider
	mu        sync.RWMutex
}

var globalRegistry = &ProviderRegistry{
	providers: make(map[string]MarketDataProvider),
}

// RegisterProvider registers a market data provider
func RegisterProvider(name string, provider MarketDataProvider) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.providers[name] = provider
}

// GetProvider returns a provider by name
func GetProvider(name string) (MarketDataProvider, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	provider, ok := globalRegistry.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found", name)
	}
	return provider, nil
}

// ListProviders returns all registered provider names
func ListProviders() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	names := make([]string, 0, len(globalRegistry.providers))
	for name := range globalRegistry.providers {
		names = append(names, name)
	}
	return names
}

var defaultProviderName = "binance"
var defaultProviderLock sync.RWMutex

// SetDefaultProviderName sets the default provider name
func SetDefaultProviderName(name string) error {
	defaultProviderLock.Lock()
	defer defaultProviderLock.Unlock()

	_, err := GetProvider(name)
	if err != nil {
		return fmt.Errorf("cannot set default provider: %w", err)
	}
	defaultProviderName = name
	return nil
}

// GetDefaultProvider returns the default provider
func GetDefaultProvider() (MarketDataProvider, error) {
	defaultProviderLock.RLock()
	name := defaultProviderName
	defaultProviderLock.RUnlock()

	return GetProvider(name)
}

// InitializeProviders registers all built-in providers
func InitializeProviders() {
	// Original providers
	RegisterProvider("binance", NewBinanceProvider())
	RegisterProvider("gateio", NewGateioProvider())
	RegisterProvider("okx", NewOKXProvider())
	RegisterProvider("bybit", NewBybitProvider())
	RegisterProvider("huobi", NewHuobiProvider())
	RegisterProvider("kucoin", NewKuCoinProvider())
	RegisterProvider("bitfinex", NewBitfinexProvider())
	RegisterProvider("coinbase", NewCoinbaseProvider())

	// Additional providers from Python aggregation module
	RegisterProvider("binance_us", NewBinanceUSProvider())
	RegisterProvider("bitstamp", NewBitstampProvider())
	RegisterProvider("bitmex", NewBitmexProvider())
	RegisterProvider("deribit", NewDeribitProvider())
	RegisterProvider("hitbtc", NewHitBTCProvider())
	RegisterProvider("bitget", NewBitgetProvider())
	RegisterProvider("mexc", NewMEXCProvider())
	RegisterProvider("crypto_com", NewCryptoComProvider())
	RegisterProvider("kraken", NewKrakenProvider())
	RegisterProvider("gemini", NewGeminiProvider())
	RegisterProvider("digifinex", NewDigifinexProvider())
	RegisterProvider("whitebit", NewWhitebitProvider())
	RegisterProvider("upbit", NewUpbitProvider())
	RegisterProvider("alpaca_crypto", NewAlpacaCryptoProvider())

	// Set binance as default
	SetDefaultProviderName("binance")
}

