package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// GateioProvider implements MarketDataProvider for Gate.io exchange
type GateioProvider struct {
	baseURL string
}

// NewGateioProvider creates a new Gate.io provider
func NewGateioProvider() *GateioProvider {
	return &GateioProvider{
		baseURL: "https://api.gateio.ws/api/v4",
	}
}

// GetName returns the provider name
func (p *GateioProvider) GetName() string {
	return "gateio"
}

// NormalizeSymbol converts symbol to Gate.io format (e.g., BTCUSDT -> BTC_USDT)
func (p *GateioProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	// Remove hyphens first
	symbol = strings.ReplaceAll(symbol, "-", "")
	// Convert to underscore format
	if strings.HasSuffix(symbol, "USDT") && len(symbol) > 4 {
		base := symbol[:len(symbol)-4]
		return base + "_USDT"
	}
	if strings.HasSuffix(symbol, "USDC") && len(symbol) > 4 {
		base := symbol[:len(symbol)-4]
		return base + "_USDC"
	}
	if strings.HasSuffix(symbol, "BTC") && len(symbol) > 3 {
		base := symbol[:len(symbol)-3]
		return base + "_BTC"
	}
	if strings.HasSuffix(symbol, "ETH") && len(symbol) > 3 {
		base := symbol[:len(symbol)-3]
		return base + "_ETH"
	}
	// If already has underscore, return as is
	if strings.Contains(symbol, "_") {
		return symbol
	}
	// Default: assume USDT pair
	return symbol + "_USDT"
}

// convertInterval converts standard interval to Gate.io format
func (p *GateioProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "1m",
		"3m":  "3m",
		"5m":  "5m",
		"15m": "15m",
		"30m": "30m",
		"1h":  "1h",
		"4h":  "4h",
		"1d":  "1d",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "1m" // Default
}

// getIntervalSeconds converts interval string to seconds
func (p *GateioProvider) getIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"1m":  60,
		"3m":  180,
		"5m":  300,
		"15m": 900,
		"30m": 1800,
		"1h":  3600,
		"4h":  14400,
		"1d":  86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 60 // Default to 1 minute
}

// GetKlines fetches candlestick data from Gate.io
func (p *GateioProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	originalSymbol := symbol
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)

	// Gate.io futures candlestick API
	// Build URL with proper query encoding
	apiURL := fmt.Sprintf("%s/futures/usdt/candlesticks?contract=%s&interval=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	log.Printf("📊 [Gate.io] 获取K线数据: %s (%s) -> %s, 间隔=%s, 数量=%d", originalSymbol, symbol, apiURL, interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("gateio klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("gateio klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gateio klines read failed: %w", err)
	}

	// Gate.io returns an array of objects: [{"o":"110209.2","v":2975828,"t":1761914160,"c":"109903.9","l":"109850","h":"110221.9","sum":"32747835.87905"}, ...]
	var rawData []map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("gateio klines parse failed: %w", err)
	}

	klines := make([]Kline, len(rawData))
	for i, item := range rawData {
		// Gate.io format: {"o":open, "v":volume, "t":timestamp, "c":close, "l":low, "h":high, "sum":quote_volume}
		// All values can be strings or numbers
		open := parseFloatSafe(item["o"])
		volume := parseFloatSafe(item["v"])
		timestamp := parseFloatSafe(item["t"])
		close := parseFloatSafe(item["c"])
		low := parseFloatSafe(item["l"])
		high := parseFloatSafe(item["h"])

		openTime := int64(timestamp * 1000) // Convert seconds to milliseconds
		// Calculate close time based on actual interval
		intervalSeconds := p.getIntervalSeconds(interval)
		closeTime := openTime + (intervalSeconds * 1000)

		klines[i] = Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		}
	}

	if len(klines) > 0 {
		latestPrice := klines[len(klines)-1].Close
		log.Printf("✓ [Gate.io] 成功获取 %s K线数据: %d根, 最新价格=%.2f", originalSymbol, len(klines), latestPrice)
	}

	return klines, nil
}

// GetOpenInterest fetches open interest data from Gate.io
func (p *GateioProvider) GetOpenInterest(symbol string) (*OIData, error) {
	originalSymbol := symbol
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/futures/usdt/contracts/%s", p.baseURL, symbol)

	log.Printf("📊 [Gate.io] 获取持仓量数据: %s -> %s", originalSymbol, symbol)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("gateio open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("gateio open interest API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gateio open interest read failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("gateio open interest parse failed: %w", err)
	}

	// Gate.io doesn't have "open_interest" field directly, use "position_size" as open interest
	oi := parseFloatSafe(result["position_size"])
	// Fallback to "open_interest" if it exists
	if oi == 0 {
		oi = parseFloatSafe(result["open_interest"])
	}

	oiData := &OIData{
		Latest:  oi,
		Average: oi * 0.999, // Approximate average
	}
	log.Printf("✓ [Gate.io] 成功获取 %s 持仓量: %.2f", originalSymbol, oi)
	return oiData, nil
}

// GetFundingRate fetches funding rate from Gate.io
func (p *GateioProvider) GetFundingRate(symbol string) (float64, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/futures/usdt/contracts/%s", p.baseURL, symbol)

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, fmt.Errorf("gateio funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return 0, fmt.Errorf("gateio funding rate API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("gateio funding rate read failed: %w", err)
	}

	var result struct {
		FundingRate interface{} `json:"funding_rate"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("gateio funding rate parse failed: %w", err)
	}

	rate := parseFloatSafe(result.FundingRate)
	return rate, nil
}

// parseFloatSafe safely parses interface{} to float64
func parseFloatSafe(v interface{}) float64 {
	switch val := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

