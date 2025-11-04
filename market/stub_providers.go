package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

// Market data provider implementations for multiple exchanges
// All providers implement the MarketDataProvider interface with:
// - GetKlines: Fetch candlestick/OHLCV data
// - GetOpenInterest: Fetch open interest data (futures/perpetual markets)
// - GetFundingRate: Fetch funding rate (futures/perpetual markets)
// - NormalizeSymbol: Convert symbols to exchange-specific format

// OKXProvider implements MarketDataProvider for OKX exchange
type OKXProvider struct {
	baseURL string
}

func NewOKXProvider() *OKXProvider {
	return &OKXProvider{
		baseURL: "https://www.okx.com/api/v5",
	}
}

func (p *OKXProvider) GetName() string {
	return "okx"
}

func (p *OKXProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "_", "-")
	if !strings.Contains(symbol, "-") && strings.HasSuffix(symbol, "USDT") {
		base := symbol[:len(symbol)-4]
		return base + "-USDT-SWAP" // OKX perpetual futures use -SWAP suffix
	}
	// If already has -USDT, convert to -USDT-SWAP for futures
	if strings.HasSuffix(symbol, "-USDT") && !strings.HasSuffix(symbol, "-SWAP") {
		return symbol + "-SWAP"
	}
	return symbol
}

func (p *OKXProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "1m",
		"3m":  "3m",
		"5m":  "5m",
		"15m": "15m",
		"30m": "30m",
		"1h":  "1H",
		"4h":  "4H",
		"1d":  "1D",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "3m" // Default
}

func (p *OKXProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	
	apiURL := fmt.Sprintf("%s/market/candles?instId=%s&bar=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("okx klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("okx klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("okx klines read failed: %w", err)
	}

	var result struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("okx klines parse failed: %w", err)
	}

	if result.Code != "0" {
		return nil, fmt.Errorf("okx API error: %s", result.Msg)
	}

	klines := make([]Kline, len(result.Data))
	for i, item := range result.Data {
		if len(item) < 6 {
			continue
		}
		// OKX format: [timestamp, open, high, low, close, volume, volumeCurrency, ...]
		openTime, _ := strconv.ParseInt(item[0], 10, 64)
		open, _ := strconv.ParseFloat(item[1], 64)
		high, _ := strconv.ParseFloat(item[2], 64)
		low, _ := strconv.ParseFloat(item[3], 64)
		close, _ := strconv.ParseFloat(item[4], 64)
		volume, _ := strconv.ParseFloat(item[5], 64)

		// Calculate close time (interval in milliseconds)
		intervalSeconds := getOKXIntervalSeconds(interval)
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

	return klines, nil
}

func (p *OKXProvider) GetOpenInterest(symbol string) (*OIData, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/public/open-interest?instId=%s", p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("okx open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("okx open interest API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("okx open interest read failed: %w", err)
	}

	var result struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			InstId      string `json:"instId"`
			Oi          string `json:"oi"`
			OiCcy       string `json:"oiCcy"`
			Time        string `json:"ts"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("okx open interest parse failed: %w", err)
	}

	if result.Code != "0" || len(result.Data) == 0 {
		return nil, fmt.Errorf("okx API error: %s", result.Msg)
	}

	oi, _ := strconv.ParseFloat(result.Data[0].Oi, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // Approximate average
	}, nil
}

func (p *OKXProvider) GetFundingRate(symbol string) (float64, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/public/funding-rate?instId=%s", p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, fmt.Errorf("okx funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return 0, fmt.Errorf("okx funding rate API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("okx funding rate read failed: %w", err)
	}

	var result struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			InstId      string `json:"instId"`
			FundingRate string `json:"fundingRate"`
			NextFundingTime string `json:"nextFundingTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("okx funding rate parse failed: %w", err)
	}

	if result.Code != "0" || len(result.Data) == 0 {
		return 0, fmt.Errorf("okx API error: %s", result.Msg)
	}

	rate, _ := strconv.ParseFloat(result.Data[0].FundingRate, 64)
	return rate, nil
}

// getOKXIntervalSeconds converts OKX interval string to seconds
func getOKXIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"1m":  60,
		"3m":  180,
		"5m":  300,
		"15m": 900,
		"30m": 1800,
		"1H":  3600,
		"4H":  14400,
		"1D":  86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 180 // Default to 3 minutes
}

// BybitProvider implements MarketDataProvider for Bybit exchange
type BybitProvider struct {
	baseURL string
}

func NewBybitProvider() *BybitProvider {
	return &BybitProvider{
		baseURL: "https://api.bybit.com/v5",
	}
}

func (p *BybitProvider) GetName() string {
	return "bybit"
}

func (p *BybitProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	return symbol
}

func (p *BybitProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "1",
		"3m":  "3",
		"5m":  "5",
		"15m": "15",
		"30m": "30",
		"1h":  "60",
		"4h":  "240",
		"1d":  "D",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "3" // Default to 3 minutes
}

func (p *BybitProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	
	apiURL := fmt.Sprintf("%s/market/kline?category=linear&symbol=%s&interval=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("bybit klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("bybit klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bybit klines read failed: %w", err)
	}

	var result struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			Symbol string     `json:"symbol"`
			List   [][]string `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("bybit klines parse failed: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("bybit API error: %s", result.RetMsg)
	}

	klines := make([]Kline, len(result.Result.List))
	for i, item := range result.Result.List {
		if len(item) < 6 {
			continue
		}
		// Bybit format: [timestamp, open, high, low, close, volume, turnover]
		openTime, _ := strconv.ParseInt(item[0], 10, 64)
		open, _ := strconv.ParseFloat(item[1], 64)
		high, _ := strconv.ParseFloat(item[2], 64)
		low, _ := strconv.ParseFloat(item[3], 64)
		close, _ := strconv.ParseFloat(item[4], 64)
		volume, _ := strconv.ParseFloat(item[5], 64)

		// Calculate close time (interval in milliseconds)
		intervalSeconds := getBybitIntervalSeconds(interval)
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

	return klines, nil
}

func (p *BybitProvider) GetOpenInterest(symbol string) (*OIData, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/market/tickers?category=linear&symbol=%s", p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("bybit open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("bybit open interest API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bybit open interest read failed: %w", err)
	}

	var result struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				Symbol      string `json:"symbol"`
				OpenInterest string `json:"openInterest"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("bybit open interest parse failed: %w", err)
	}

	if result.RetCode != 0 || len(result.Result.List) == 0 {
		return nil, fmt.Errorf("bybit API error: %s", result.RetMsg)
	}

	oi, _ := strconv.ParseFloat(result.Result.List[0].OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // Approximate average
	}, nil
}

func (p *BybitProvider) GetFundingRate(symbol string) (float64, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/market/tickers?category=linear&symbol=%s", p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, fmt.Errorf("bybit funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return 0, fmt.Errorf("bybit funding rate API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("bybit funding rate read failed: %w", err)
	}

	var result struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				Symbol      string `json:"symbol"`
				FundingRate string `json:"fundingRate"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("bybit funding rate parse failed: %w", err)
	}

	if result.RetCode != 0 || len(result.Result.List) == 0 {
		return 0, fmt.Errorf("bybit API error: %s", result.RetMsg)
	}

	rate, _ := strconv.ParseFloat(result.Result.List[0].FundingRate, 64)
	return rate, nil
}

// getBybitIntervalSeconds converts Bybit interval string to seconds
func getBybitIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"1":   60,
		"3":   180,
		"5":   300,
		"15":  900,
		"30":  1800,
		"60":  3600,
		"240": 14400,
		"D":   86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 180 // Default to 3 minutes
}

// HuobiProvider implements MarketDataProvider for Huobi exchange
type HuobiProvider struct {
	baseURL string
}

func NewHuobiProvider() *HuobiProvider {
	return &HuobiProvider{
		baseURL: "https://api.huobi.pro",
	}
}

func (p *HuobiProvider) GetName() string {
	return "huobi"
}

func (p *HuobiProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	return strings.ToLower(symbol)
}

func (p *HuobiProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "1min",
		"3m":  "3min",
		"5m":  "5min",
		"15m": "15min",
		"30m": "30min",
		"1h":  "60min",
		"4h":  "4hour",
		"1d":  "1day",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "3min" // Default
}

func (p *HuobiProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	
	apiURL := fmt.Sprintf("%s/market/history/kline?symbol=%s&period=%s&size=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("huobi klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("huobi klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("huobi klines read failed: %w", err)
	}

	var result struct {
		Status string `json:"status"`
		Data   []struct {
			ID     int64   `json:"id"` // Unix timestamp in seconds
			Open   float64 `json:"open"`
			High   float64 `json:"high"`
			Low    float64 `json:"low"`
			Close  float64 `json:"close"`
			Amount float64 `json:"amount"` // Volume in base currency
			Vol    float64 `json:"vol"`    // Volume in quote currency
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("huobi klines parse failed: %w", err)
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("huobi API error: status=%s", result.Status)
	}

	klines := make([]Kline, len(result.Data))
	for i, item := range result.Data {
		openTime := item.ID * 1000 // Convert seconds to milliseconds
		intervalSeconds := getHuobiIntervalSeconds(interval)
		closeTime := openTime + (intervalSeconds * 1000)

		klines[i] = Kline{
			OpenTime:  openTime,
			Open:      item.Open,
			High:      item.High,
			Low:       item.Low,
			Close:     item.Close,
			Volume:    item.Vol, // Use quote currency volume
			CloseTime: closeTime,
		}
	}

	return klines, nil
}

func (p *HuobiProvider) GetOpenInterest(symbol string) (*OIData, error) {
	// Huobi linear swap API for open interest
	// Try with different symbol format - Huobi uses BTC-USDT for futures
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "USDT", "-USDT")
	if !strings.Contains(symbol, "-") {
		// Add dash if missing
		if strings.HasSuffix(symbol, "USDT") {
			base := symbol[:len(symbol)-4]
			symbol = base + "-USDT"
		}
	}
	
	apiURL := fmt.Sprintf("%s/linear-swap-api/v1/swap_open_interest?contract_code=%s",
		p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("huobi open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("huobi open interest read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("huobi open interest API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Status string `json:"status"`
		Data   []struct {
			Symbol      string  `json:"symbol"`
			ContractCode string `json:"contract_code"`
			Volume      float64 `json:"volume"`
			Amount      float64 `json:"amount"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("huobi open interest parse failed: %w", err)
	}

	if result.Status != "ok" || len(result.Data) == 0 {
		return nil, fmt.Errorf("huobi API error: status=%s", result.Status)
	}

	oi := result.Data[0].Volume

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // Approximate average
	}, nil
}

func (p *HuobiProvider) GetFundingRate(symbol string) (float64, error) {
	// Huobi linear swap API for funding rate
	// Try with different symbol format
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "USDT", "-USDT")
	if !strings.Contains(symbol, "-") {
		// Add dash if missing
		if strings.HasSuffix(symbol, "USDT") {
			base := symbol[:len(symbol)-4]
			symbol = base + "-USDT"
		}
	}
	
	apiURL := fmt.Sprintf("%s/linear-swap-api/v1/swap_funding_rate?contract_code=%s",
		p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, fmt.Errorf("huobi funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("huobi funding rate read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("huobi funding rate API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Status string `json:"status"`
		Data   []struct {
			Symbol       string  `json:"symbol"`
			ContractCode string  `json:"contract_code"`
			FundingRate  float64 `json:"funding_rate"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("huobi funding rate parse failed: %w", err)
	}

	if result.Status != "ok" || len(result.Data) == 0 {
		return 0, fmt.Errorf("huobi API error: status=%s", result.Status)
	}

	return result.Data[0].FundingRate, nil
}

// getHuobiIntervalSeconds converts Huobi interval string to seconds
func getHuobiIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"1min":  60,
		"3min":  180,
		"5min":  300,
		"15min": 900,
		"30min": 1800,
		"60min": 3600,
		"4hour": 14400,
		"1day":  86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 180 // Default to 3 minutes
}

// KuCoinProvider implements MarketDataProvider for KuCoin exchange
type KuCoinProvider struct {
	spotBaseURL    string
	futuresBaseURL string
}

func NewKuCoinProvider() *KuCoinProvider {
	return &KuCoinProvider{
		spotBaseURL:    "https://api.kucoin.com/api/v1",
		futuresBaseURL: "https://api-futures.kucoin.com/api/v1",
	}
}

func (p *KuCoinProvider) GetName() string {
	return "kucoin"
}

func (p *KuCoinProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "_", "-")
	if !strings.Contains(symbol, "-") && strings.HasSuffix(symbol, "USDT") {
		base := symbol[:len(symbol)-4]
		return base + "-USDT"
	}
	return symbol
}

func (p *KuCoinProvider) normalizeFuturesSymbol(symbol string) string {
	// KuCoin futures uses BTCUSDTM format
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	if strings.HasSuffix(symbol, "USDT") && !strings.HasSuffix(symbol, "USDTM") {
		return symbol + "M" // Add M suffix for perpetual futures
	}
	return symbol
}

func (p *KuCoinProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "1min",
		"3m":  "3min",
		"5m":  "5min",
		"15m": "15min",
		"30m": "30min",
		"1h":  "1hour",
		"4h":  "4hour",
		"1d":  "1day",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "3min" // Default
}

func (p *KuCoinProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	
	// KuCoin API - get recent candles (returns oldest first, so we'll reverse)
	apiURL := fmt.Sprintf("%s/market/candles?type=%s&symbol=%s",
		p.spotBaseURL, interval, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("kucoin klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("kucoin klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kucoin klines read failed: %w", err)
	}

	var result struct {
		Code string     `json:"code"`
		Data [][]string `json:"data"` // KuCoin returns strings: [time, open, close, high, low, volume, turnover]
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("kucoin klines parse failed: %w", err)
	}

	if result.Code != "200000" {
		return nil, fmt.Errorf("kucoin API error: code=%s", result.Code)
	}

	// KuCoin returns data in reverse chronological order (oldest first)
	// We need to take the last 'limit' items and reverse them
	startIdx := 0
	if len(result.Data) > limit {
		startIdx = len(result.Data) - limit
	}
	recentData := result.Data[startIdx:]

	klines := make([]Kline, 0, len(recentData))
	// Process in reverse to get most recent first
	for i := len(recentData) - 1; i >= 0; i-- {
		item := recentData[i]
		if len(item) < 6 {
			continue
		}
		// KuCoin format: [time, open, close, high, low, volume, turnover]
		// Time is in seconds (Unix timestamp), need to convert to milliseconds
		openTimeSeconds, _ := strconv.ParseInt(item[0], 10, 64)
		openTime := openTimeSeconds * 1000 // Convert to milliseconds
		open, _ := strconv.ParseFloat(item[1], 64)
		close, _ := strconv.ParseFloat(item[2], 64)
		high, _ := strconv.ParseFloat(item[3], 64)
		low, _ := strconv.ParseFloat(item[4], 64)
		volume, _ := strconv.ParseFloat(item[5], 64)

		intervalSeconds := getKuCoinIntervalSeconds(interval)
		closeTime := openTime + (intervalSeconds * 1000)

		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}

func (p *KuCoinProvider) GetOpenInterest(symbol string) (*OIData, error) {
	// KuCoin futures API
	symbol = p.normalizeFuturesSymbol(symbol)
	apiURL := fmt.Sprintf("%s/openInterest?symbol=%s", p.futuresBaseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("kucoin open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kucoin open interest read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kucoin open interest API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code string `json:"code"`
		Data struct {
			OpenInterest float64 `json:"openInterest"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("kucoin open interest parse failed: %w", err)
	}

	if result.Code != "200000" {
		return nil, fmt.Errorf("kucoin API error: code=%s", result.Code)
	}

	oi := result.Data.OpenInterest

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // Approximate average
	}, nil
}

func (p *KuCoinProvider) GetFundingRate(symbol string) (float64, error) {
	// KuCoin futures API
	symbol = p.normalizeFuturesSymbol(symbol)
	apiURL := fmt.Sprintf("%s/funding-rate?symbol=%s", p.futuresBaseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, fmt.Errorf("kucoin funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("kucoin funding rate read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("kucoin funding rate API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code string `json:"code"`
		Data struct {
			FundingRate float64 `json:"fundingRate"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("kucoin funding rate parse failed: %w", err)
	}

	if result.Code != "200000" {
		return 0, fmt.Errorf("kucoin API error: code=%s", result.Code)
	}

	return result.Data.FundingRate, nil
}

// getKuCoinIntervalSeconds converts KuCoin interval string to seconds
func getKuCoinIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"1min":  60,
		"3min":  180,
		"5min":  300,
		"15min": 900,
		"30min": 1800,
		"1hour": 3600,
		"4hour": 14400,
		"1day":  86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 180 // Default to 3 minutes
}

// BitfinexProvider implements MarketDataProvider for Bitfinex exchange
// Note: Bitfinex is primarily a spot exchange, futures/OI may have limited support
type BitfinexProvider struct {
	baseURL string
}

func NewBitfinexProvider() *BitfinexProvider {
	return &BitfinexProvider{
		baseURL: "https://api.bitfinex.com/v2",
	}
}

func (p *BitfinexProvider) GetName() string {
	return "bitfinex"
}

func (p *BitfinexProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "USDT", "USD")
	return "t" + symbol
}

func (p *BitfinexProvider) convertInterval(interval string) string {
	// Bitfinex format: 1m, 5m, 15m, 30m, 1h, 3h, 6h, 12h, 1D, 7D, 14D, 1M
	intervalMap := map[string]string{
		"1m":  "1m",
		"3m":  "3m",
		"5m":  "5m",
		"15m": "15m",
		"30m": "30m",
		"1h":  "1h",
		"4h":  "4h",
		"1d":  "1D",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "3m" // Default
}

func (p *BitfinexProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	
	// Bitfinex requires sort=1 to get most recent first
	apiURL := fmt.Sprintf("%s/candles/trade:%s:%s/hist?limit=%d&sort=1",
		p.baseURL, interval, url.QueryEscape(symbol), limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("bitfinex klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("bitfinex klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bitfinex klines read failed: %w", err)
	}

	// Bitfinex returns array of arrays: [[timestamp, open, close, high, low, volume], ...]
	var rawData [][]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("bitfinex klines parse failed: %w", err)
	}

	klines := make([]Kline, 0, len(rawData))
	for _, item := range rawData {
		if len(item) < 6 {
			continue
		}
		// Bitfinex format: [timestamp, open, close, high, low, volume]
		openTime := int64(item[0].(float64))
		
		// Use parseFloat from data.go (same package)
		open, _ := parseFloat(item[1])
		close, _ := parseFloat(item[2])
		high, _ := parseFloat(item[3])
		low, _ := parseFloat(item[4])
		volume, _ := parseFloat(item[5])

		intervalSeconds := getBitfinexIntervalSeconds(interval)
		closeTime := openTime + (intervalSeconds * 1000)

		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}

func (p *BitfinexProvider) GetOpenInterest(symbol string) (*OIData, error) {
	// Bitfinex doesn't have a public open interest endpoint for spot trading
	// This is primarily a spot exchange, so we return a not implemented error
	return nil, fmt.Errorf("Bitfinex is primarily a spot exchange; open interest not available via public API")
}

func (p *BitfinexProvider) GetFundingRate(symbol string) (float64, error) {
	// Bitfinex doesn't have funding rates for spot trading
	return 0, fmt.Errorf("Bitfinex is primarily a spot exchange; funding rate not available")
}

// getBitfinexIntervalSeconds converts Bitfinex interval string to seconds
func getBitfinexIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"1m":  60,
		"3m":  180,
		"5m":  300,
		"15m": 900,
		"30m": 1800,
		"1h":  3600,
		"4h":  14400,
		"1D":  86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 180 // Default to 3 minutes
}

// CoinbaseProvider implements MarketDataProvider for Coinbase exchange
// Note: Coinbase is a spot-only exchange, no futures/open interest/funding rates
type CoinbaseProvider struct {
	baseURL string
}

func NewCoinbaseProvider() *CoinbaseProvider {
	return &CoinbaseProvider{
		baseURL: "https://api.coinbase.com/api/v3/brokerage",
	}
}

func (p *CoinbaseProvider) GetName() string {
	return "coinbase"
}

func (p *CoinbaseProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "USDT", "USD")
	symbol = strings.ReplaceAll(symbol, "_", "-")
	if !strings.Contains(symbol, "-") {
		// Assume BTCUSDT -> BTC-USD
		if strings.HasSuffix(symbol, "USD") {
			base := symbol[:len(symbol)-3]
			return base + "-USD"
		}
		// If no USD suffix, add it
		return symbol + "-USD"
	}
	return symbol
}

func (p *CoinbaseProvider) convertInterval(interval string) string {
	// Coinbase public API granularity (in seconds): 60, 300, 900, 3600, 21600, 86400
	// Map to closest supported granularity
	intervalMap := map[string]int64{
		"1m":  60,   // 1 minute -> 60 seconds
		"3m":  300,  // 3 minutes -> use 5 minutes (300 seconds) as closest
		"5m":  300,  // 5 minutes -> 300 seconds
		"15m": 900,  // 15 minutes -> 900 seconds
		"30m": 900,  // 30 minutes -> use 15 minutes (900 seconds) as closest
		"1h":  3600, // 1 hour -> 3600 seconds
		"4h":  21600, // 4 hours -> use 6 hours (21600 seconds) as closest
		"1d":  86400, // 1 day -> 86400 seconds
	}
	if seconds, ok := intervalMap[interval]; ok {
		return fmt.Sprintf("%d", seconds)
	}
	return "300" // Default to 5 minutes
}

func (p *CoinbaseProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	granularityStr := p.convertInterval(interval) // Returns granularity in seconds as string
	
	// Public API endpoint (no auth required for historical data)
	apiURL := fmt.Sprintf("https://api.exchange.coinbase.com/products/%s/candles?granularity=%s",
		url.QueryEscape(symbol), granularityStr)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("coinbase klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("coinbase klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("coinbase klines read failed: %w", err)
	}

	// Coinbase public API returns: [[time, low, high, open, close, volume], ...]
	var rawData [][]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("coinbase klines parse failed: %w", err)
	}

	// Take only the requested limit (most recent)
	startIdx := 0
	if len(rawData) > limit {
		startIdx = len(rawData) - limit
	}
	recentData := rawData[startIdx:]

	klines := make([]Kline, 0, len(recentData))
	// Process in reverse to get most recent first (Coinbase returns oldest first)
	for i := len(recentData) - 1; i >= 0; i-- {
		item := recentData[i]
		if len(item) < 6 {
			continue
		}
		// Coinbase format: [time, low, high, open, close, volume]
		openTime := int64(item[0].(float64)) * 1000 // Convert seconds to milliseconds
		low, _ := parseFloat(item[1])
		high, _ := parseFloat(item[2])
		open, _ := parseFloat(item[3])
		close, _ := parseFloat(item[4])
		volume, _ := parseFloat(item[5])

		// Calculate interval seconds from granularity
		granularitySeconds, _ := strconv.ParseInt(granularityStr, 10, 64)
		closeTime := openTime + (granularitySeconds * 1000)

		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}

func (p *CoinbaseProvider) GetOpenInterest(symbol string) (*OIData, error) {
	// Coinbase is a spot-only exchange, no open interest
	return nil, fmt.Errorf("Coinbase is a spot-only exchange; open interest not available")
}

func (p *CoinbaseProvider) GetFundingRate(symbol string) (float64, error) {
	// Coinbase is a spot-only exchange, no funding rates
	return 0, fmt.Errorf("Coinbase is a spot-only exchange; funding rate not available")
}

// ============================================================================
// Additional Exchange Providers (from Python aggregation module)
// ============================================================================

// BinanceUSProvider implements MarketDataProvider for Binance US exchange
type BinanceUSProvider struct {
	baseURL string
}

func NewBinanceUSProvider() *BinanceUSProvider {
	return &BinanceUSProvider{
		baseURL: "https://api.binance.us/api/v3",
	}
}

func (p *BinanceUSProvider) GetName() string {
	return "binance_us"
}

func (p *BinanceUSProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	return symbol // Binance US requires uppercase, not lowercase
}

func (p *BinanceUSProvider) convertInterval(interval string) string {
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
	return "1m"
}

func (p *BinanceUSProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/klines?symbol=%s&interval=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("binance_us klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance_us klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("binance_us klines read failed: %w", err)
	}

	var rawData [][]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("binance_us klines parse failed: %w", err)
	}

	klines := make([]Kline, len(rawData))
	for i, item := range rawData {
		openTime := int64(item[0].(float64))
		open, _ := parseFloat(item[1])
		high, _ := parseFloat(item[2])
		low, _ := parseFloat(item[3])
		close, _ := parseFloat(item[4])
		volume, _ := parseFloat(item[5])
		closeTime := int64(item[6].(float64))

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

	return klines, nil
}

func (p *BinanceUSProvider) GetOpenInterest(symbol string) (*OIData, error) {
	return nil, fmt.Errorf("Binance US is spot-only; open interest not available")
}

func (p *BinanceUSProvider) GetFundingRate(symbol string) (float64, error) {
	return 0, fmt.Errorf("Binance US is spot-only; funding rate not available")
}

// BitstampProvider implements MarketDataProvider for Bitstamp exchange
type BitstampProvider struct {
	baseURL string
}

func NewBitstampProvider() *BitstampProvider {
	return &BitstampProvider{
		baseURL: "https://www.bitstamp.net/api/v2",
	}
}

func (p *BitstampProvider) GetName() string {
	return "bitstamp"
}

func (p *BitstampProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "USDT", "USD")
	return strings.ToLower(symbol)
}

func (p *BitstampProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "60",
		"3m":  "180",
		"5m":  "300",
		"15m": "900",
		"30m": "1800",
		"1h":  "3600",
		"4h":  "14400",
		"1d":  "86400",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "300"
}

func (p *BitstampProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/ohlc/%s/?step=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("bitstamp klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("bitstamp klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bitstamp klines read failed: %w", err)
	}

	var result struct {
		Data struct {
			OhlcData []struct {
				Timestamp string `json:"timestamp"`
				Open      string `json:"open"`
				High      string `json:"high"`
				Low       string `json:"low"`
				Close     string `json:"close"`
				Volume    string `json:"volume"`
			} `json:"ohlc"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("bitstamp klines parse failed: %w", err)
	}

	klines := make([]Kline, 0, len(result.Data.OhlcData))
	for _, item := range result.Data.OhlcData {
		openTime, _ := strconv.ParseInt(item.Timestamp, 10, 64)
		open, _ := strconv.ParseFloat(item.Open, 64)
		high, _ := strconv.ParseFloat(item.High, 64)
		low, _ := strconv.ParseFloat(item.Low, 64)
		close, _ := strconv.ParseFloat(item.Close, 64)
		volume, _ := strconv.ParseFloat(item.Volume, 64)

		intervalSeconds, _ := strconv.ParseInt(interval, 10, 64)
		closeTime := openTime*1000 + (intervalSeconds * 1000)

		klines = append(klines, Kline{
			OpenTime:  openTime * 1000,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}

func (p *BitstampProvider) GetOpenInterest(symbol string) (*OIData, error) {
	return nil, fmt.Errorf("Bitstamp is spot-only; open interest not available")
}

func (p *BitstampProvider) GetFundingRate(symbol string) (float64, error) {
	return 0, fmt.Errorf("Bitstamp is spot-only; funding rate not available")
}

// BitmexProvider implements MarketDataProvider for BitMEX exchange
type BitmexProvider struct {
	baseURL string
}

func NewBitmexProvider() *BitmexProvider {
	return &BitmexProvider{
		baseURL: "https://www.bitmex.com/api/v1",
	}
}

func (p *BitmexProvider) GetName() string {
	return "bitmex"
}

func (p *BitmexProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.Contains(symbol, "BTC") {
		return "XBTUSD" // BitMEX uses XBT for Bitcoin
	}
	return symbol
}

func (p *BitmexProvider) convertInterval(interval string) string {
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
	return "5m"
}

func (p *BitmexProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/trade/bucketed?symbol=%s&binSize=%s&count=%d&reverse=true",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("bitmex klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("bitmex klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bitmex klines read failed: %w", err)
	}

	var rawData []map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("bitmex klines parse failed: %w", err)
	}

	klines := make([]Kline, 0, len(rawData))
	for i := len(rawData) - 1; i >= 0; i-- {
		item := rawData[i]
		openTimeStr, _ := item["timestamp"].(string)
		openTime, _ := parseTimeRFC3339(openTimeStr)
		open := parseFloatSafe(item["open"])
		high := parseFloatSafe(item["high"])
		low := parseFloatSafe(item["low"])
		close := parseFloatSafe(item["close"])
		volume := parseFloatSafe(item["volume"])

		intervalSeconds := getBitmexIntervalSeconds(interval)
		closeTime := openTime + (intervalSeconds * 1000)

		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}

func (p *BitmexProvider) GetOpenInterest(symbol string) (*OIData, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/instrument?symbol=%s", p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("bitmex open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bitmex open interest API error (status %d)", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bitmex open interest read failed: %w", err)
	}

	var rawData []map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("bitmex open interest parse failed: %w", err)
	}

	if len(rawData) == 0 {
		return nil, fmt.Errorf("bitmex no data returned")
	}

	oi := parseFloatSafe(rawData[0]["openInterest"])

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999,
	}, nil
}

func (p *BitmexProvider) GetFundingRate(symbol string) (float64, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/instrument?symbol=%s", p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, fmt.Errorf("bitmex funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bitmex funding rate API error (status %d)", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("bitmex funding rate read failed: %w", err)
	}

	var rawData []map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return 0, fmt.Errorf("bitmex funding rate parse failed: %w", err)
	}

	if len(rawData) == 0 {
		return 0, fmt.Errorf("bitmex no data returned")
	}

	return parseFloatSafe(rawData[0]["fundingRate"]), nil
}

func getBitmexIntervalSeconds(interval string) int64 {
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
	return 300
}

// DeribitProvider implements MarketDataProvider for Deribit exchange
type DeribitProvider struct {
	baseURL string
}

func NewDeribitProvider() *DeribitProvider {
	return &DeribitProvider{
		baseURL: "https://www.deribit.com/api/v2",
	}
}

func (p *DeribitProvider) GetName() string {
	return "deribit"
}

func (p *DeribitProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "USDT", "")
	symbol = strings.ReplaceAll(symbol, "_", "-")
	if !strings.Contains(symbol, "-") {
		return symbol + "-PERPETUAL"
	}
	if !strings.HasSuffix(symbol, "-PERPETUAL") {
		return symbol + "-PERPETUAL"
	}
	return symbol
}

func (p *DeribitProvider) convertInterval(interval string) string {
	// Deribit resolution format: minutes as string (e.g., "5" for 5 minutes, "60" for 1 hour)
	intervalMap := map[string]string{
		"1m":  "1",
		"3m":  "3",
		"5m":  "5",
		"15m": "15",
		"30m": "30",
		"1h":  "60",
		"4h":  "240",
		"1d":  "1440", // 24 hours * 60 minutes
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "5"
}

func (p *DeribitProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	intervalMinutes := p.convertInterval(interval) // Now returns minutes as string
	endTime := int64(time.Now().Unix() * 1000)
	intervalMinutesInt, _ := strconv.ParseInt(intervalMinutes, 10, 64)
	startTime := endTime - (int64(limit) * intervalMinutesInt * 60 * 1000)

	apiURL := fmt.Sprintf("%s/public/get_tradingview_chart_data?instrument_name=%s&resolution=%s&start_timestamp=%d&end_timestamp=%d",
		p.baseURL, url.QueryEscape(symbol), intervalMinutes, startTime, endTime)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("deribit klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("deribit klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("deribit klines read failed: %w", err)
	}

	var result struct {
		Result struct {
			Volume []float64 `json:"volume"`
			Ticks  []int64   `json:"ticks"`
			Open   []float64 `json:"open"`
			High   []float64 `json:"high"`
			Low    []float64 `json:"low"`
			Close  []float64 `json:"close"`
			Status string    `json:"status"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("deribit klines parse failed: %w", err)
	}

	if result.Result.Status != "ok" {
		return nil, fmt.Errorf("deribit API error: status=%s", result.Result.Status)
	}

	dataLen := len(result.Result.Ticks)
	klines := make([]Kline, dataLen)
	intervalSeconds := intervalMinutesInt * 60
	for i := 0; i < dataLen; i++ {
			openTime := result.Result.Ticks[i]
			closeTime := openTime + (intervalSeconds * 1000)

		klines[i] = Kline{
			OpenTime:  openTime,
			Open:      result.Result.Open[i],
			High:      result.Result.High[i],
			Low:       result.Result.Low[i],
			Close:     result.Result.Close[i],
			Volume:    result.Result.Volume[i],
			CloseTime: closeTime,
		}
	}

	return klines, nil
}

func (p *DeribitProvider) GetOpenInterest(symbol string) (*OIData, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/public/get_book_summary_by_instrument?instrument_name=%s",
		p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("deribit open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("deribit open interest read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deribit open interest API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Result []struct {
			OpenInterest float64 `json:"open_interest"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("deribit open interest parse failed: %w", err)
	}

	if len(result.Result) == 0 {
		return nil, fmt.Errorf("deribit no data returned")
	}

	oi := result.Result[0].OpenInterest

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999,
	}, nil
}

func (p *DeribitProvider) GetFundingRate(symbol string) (float64, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/public/get_funding_rate_value?instrument_name=%s",
		p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, fmt.Errorf("deribit funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("deribit funding rate read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("deribit funding rate API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Result float64 `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("deribit funding rate parse failed: %w", err)
	}

	return result.Result, nil
}

func getDeribitIntervalSeconds(interval string) int64 {
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
	return 300
}

// Helper functions for parsing (used by multiple providers)

// parseTimeRFC3339 parses RFC3339 time string to Unix milliseconds
func parseTimeRFC3339(timeStr string) (int64, error) {
	if timeStr == "" {
		return 0, fmt.Errorf("empty time string")
	}
	// Try RFC3339 format first
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		// Try RFC3339Nano
		t, err = time.Parse(time.RFC3339Nano, timeStr)
		if err != nil {
			// Try simplified format
			t, err = time.Parse("2006-01-02T15:04:05.000Z", timeStr)
			if err != nil {
				return 0, fmt.Errorf("failed to parse time: %w", err)
			}
		}
	}
	return t.UnixMilli(), nil
}

// HitBTCProvider implements MarketDataProvider for HitBTC exchange
type HitBTCProvider struct {
	baseURL string
}

func NewHitBTCProvider() *HitBTCProvider {
	return &HitBTCProvider{
		baseURL: "https://api.hitbtc.com/api/3",
	}
}

func (p *HitBTCProvider) GetName() string {
	return "hitbtc"
}

func (p *HitBTCProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	// HitBTC uses USDT pairs, not USD - keep USDT
	// symbol = strings.ReplaceAll(symbol, "USDT", "USD")
	symbol = strings.ReplaceAll(symbol, "_", "")
	return symbol
}

func (p *HitBTCProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "M1",
		"3m":  "M3",
		"5m":  "M5",
		"15m": "M15",
		"30m": "M30",
		"1h":  "H1",
		"4h":  "H4",
		"1d":  "D1",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "M5"
}

func (p *HitBTCProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/public/candles/%s?periods=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("hitbtc klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("hitbtc klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hitbtc klines read failed: %w", err)
	}

	var rawData []struct {
		Timestamp  string  `json:"timestamp"`
		Open       string  `json:"open"`
		Close      string  `json:"close"`
		Min        string  `json:"min"`
		Max        string  `json:"max"`
		Volume     string  `json:"volume"`
		VolumeQuote string `json:"volume_quote"`
	}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("hitbtc klines parse failed: %w", err)
	}

	klines := make([]Kline, 0, len(rawData))
	for _, item := range rawData {
		openTime, _ := time.Parse(time.RFC3339, item.Timestamp)
		open, _ := strconv.ParseFloat(item.Open, 64)
		high, _ := strconv.ParseFloat(item.Max, 64)
		low, _ := strconv.ParseFloat(item.Min, 64)
		close, _ := strconv.ParseFloat(item.Close, 64)
		volume, _ := strconv.ParseFloat(item.Volume, 64)

		intervalSeconds := getHitBTCIntervalSeconds(interval)
		closeTime := openTime.UnixMilli() + (intervalSeconds * 1000)

		klines = append(klines, Kline{
			OpenTime:  openTime.UnixMilli(),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}

func (p *HitBTCProvider) GetOpenInterest(symbol string) (*OIData, error) {
	return nil, fmt.Errorf("HitBTC is spot-only; open interest not available")
}

func (p *HitBTCProvider) GetFundingRate(symbol string) (float64, error) {
	return 0, fmt.Errorf("HitBTC is spot-only; funding rate not available")
}

func getHitBTCIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"M1":  60,
		"M3":  180,
		"M5":  300,
		"M15": 900,
		"M30": 1800,
		"H1":  3600,
		"H4":  14400,
		"D1":  86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 300
}

// BitgetProvider implements MarketDataProvider for Bitget exchange
type BitgetProvider struct {
	baseURL string
}

func NewBitgetProvider() *BitgetProvider {
	return &BitgetProvider{
		baseURL: "https://api.bitget.com/api/v2",
	}
}

func (p *BitgetProvider) GetName() string {
	return "bitget"
}

func (p *BitgetProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	return symbol
}

func (p *BitgetProvider) convertInterval(interval string) string {
	// Bitget requires format: 1min, 3min, 5min, 15min, 30min, 1h, 4h, 6h, 12h, 1day, 1week, 1M
	intervalMap := map[string]string{
		"1m":  "1min",
		"3m":  "3min",
		"5m":  "5min",
		"15m": "15min",
		"30m": "30min",
		"1h":  "1h",
		"4h":  "4h",
		"1d":  "1day",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "5min"
}

func (p *BitgetProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/spot/market/candles?symbol=%s&granularity=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("bitget klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("bitget klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bitget klines read failed: %w", err)
	}

	var result struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("bitget klines parse failed: %w", err)
	}

	if result.Code != "00000" {
		return nil, fmt.Errorf("bitget API error: %s", result.Msg)
	}

	klines := make([]Kline, len(result.Data))
	for i, item := range result.Data {
		if len(item) < 6 {
			continue
		}
		// Bitget format: [timestamp, open, high, low, close, volume]
		openTime, _ := strconv.ParseInt(item[0], 10, 64)
		open, _ := strconv.ParseFloat(item[1], 64)
		high, _ := strconv.ParseFloat(item[2], 64)
		low, _ := strconv.ParseFloat(item[3], 64)
		close, _ := strconv.ParseFloat(item[4], 64)
		volume, _ := strconv.ParseFloat(item[5], 64)

		intervalSeconds := getBitgetIntervalSeconds(interval)
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

	return klines, nil
}

func (p *BitgetProvider) GetOpenInterest(symbol string) (*OIData, error) {
	// Bitget futures API
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/mix/market/open-interest?symbol=%s&productType=USDT-FUTURES",
		p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("bitget open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bitget open interest API error (status %d)", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bitget open interest read failed: %w", err)
	}

	var result struct {
		Code string `json:"code"`
		Data struct {
			OpenInterest string `json:"openInterest"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("bitget open interest parse failed: %w", err)
	}

	if result.Code != "00000" {
		return nil, fmt.Errorf("bitget API error: code=%s", result.Code)
	}

	oi, _ := strconv.ParseFloat(result.Data.OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999,
	}, nil
}

func (p *BitgetProvider) GetFundingRate(symbol string) (float64, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/mix/market/current-fund-rate?symbol=%s&productType=USDT-FUTURES",
		p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, fmt.Errorf("bitget funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("bitget funding rate read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bitget funding rate API error (status %d)", resp.StatusCode)
	}

	var result struct {
		Code string `json:"code"`
		Data struct {
			FundingRate string `json:"fundingRate"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("bitget funding rate parse failed: %w", err)
	}

	if result.Code != "00000" {
		return 0, fmt.Errorf("bitget API error: code=%s", result.Code)
	}

	rate, _ := strconv.ParseFloat(result.Data.FundingRate, 64)
	return rate, nil
}

func getBitgetIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"1":   60,
		"3":   180,
		"5":   300,
		"15":  900,
		"30":  1800,
		"60":  3600,
		"240": 14400,
		"1D":  86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 300
}

// MEXCProvider implements MarketDataProvider for MEXC exchange
type MEXCProvider struct {
	baseURL string
}

func NewMEXCProvider() *MEXCProvider {
	return &MEXCProvider{
		baseURL: "https://contract.mexc.com/api/v1",
	}
}

func (p *MEXCProvider) GetName() string {
	return "mexc"
}

func (p *MEXCProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "-", "_")
	if !strings.Contains(symbol, "_") && strings.HasSuffix(symbol, "USDT") {
		base := symbol[:len(symbol)-4]
		return base + "_USDT"
	}
	return symbol
}

func (p *MEXCProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "Min1",
		"3m":  "Min3",
		"5m":  "Min5",
		"15m": "Min15",
		"30m": "Min30",
		"1h":  "Hour1",
		"4h":  "Hour4",
		"1d":  "Day1",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "Min5"
}

func (p *MEXCProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/contract/kline/%s?interval=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("mexc klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("mexc klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mexc klines read failed: %w", err)
	}

	var result struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    struct {
			Time   []int64   `json:"time"`
			Open   []float64 `json:"open"`
			High   []float64 `json:"high"`
			Low    []float64 `json:"low"`
			Close  []float64 `json:"close"`
			Volume []float64 `json:"vol"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("mexc klines parse failed: %w", err)
	}

	if result.Code != 0 || !result.Success {
		return nil, fmt.Errorf("mexc API error: code=%d", result.Code)
	}

	dataLen := len(result.Data.Time)
	klines := make([]Kline, dataLen)
	intervalSeconds := getMEXCIntervalSeconds(interval)
	for i := 0; i < dataLen; i++ {
		openTime := result.Data.Time[i] * 1000 // Convert seconds to milliseconds
		closeTime := openTime + (intervalSeconds * 1000)

		klines[i] = Kline{
			OpenTime:  openTime,
			Open:      result.Data.Open[i],
			High:      result.Data.High[i],
			Low:       result.Data.Low[i],
			Close:     result.Data.Close[i],
			Volume:    result.Data.Volume[i],
			CloseTime: closeTime,
		}
	}

	return klines, nil
}

func (p *MEXCProvider) GetOpenInterest(symbol string) (*OIData, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/contract/open_interest/%s", p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("mexc open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mexc open interest API error (status %d)", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mexc open interest read failed: %w", err)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			OpenInterest float64 `json:"openInterest"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("mexc open interest parse failed: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("mexc API error: code=%d", result.Code)
	}

	return &OIData{
		Latest:  result.Data.OpenInterest,
		Average: result.Data.OpenInterest * 0.999,
	}, nil
}

func (p *MEXCProvider) GetFundingRate(symbol string) (float64, error) {
	symbol = p.NormalizeSymbol(symbol)
	apiURL := fmt.Sprintf("%s/contract/funding_rate/%s", p.baseURL, url.QueryEscape(symbol))

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, fmt.Errorf("mexc funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("mexc funding rate API error (status %d)", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("mexc funding rate read failed: %w", err)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			FundingRate float64 `json:"fundingRate"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("mexc funding rate parse failed: %w", err)
	}

	if result.Code != 0 {
		return 0, fmt.Errorf("mexc API error: code=%d", result.Code)
	}

	return result.Data.FundingRate, nil
}

func getMEXCIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"Min1":  60,
		"Min3":  180,
		"Min5":  300,
		"Min15": 900,
		"Min30": 1800,
		"Hour1": 3600,
		"Hour4": 14400,
		"Day1":  86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 300
}

// CryptoComProvider implements MarketDataProvider for Crypto.com exchange
type CryptoComProvider struct {
	baseURL string
}

func NewCryptoComProvider() *CryptoComProvider {
	return &CryptoComProvider{
		baseURL: "https://api.crypto.com/v2",
	}
}

func (p *CryptoComProvider) GetName() string {
	return "crypto_com"
}

func (p *CryptoComProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "-", "_")
	if !strings.Contains(symbol, "_") && strings.HasSuffix(symbol, "USDT") {
		base := symbol[:len(symbol)-4]
		return base + "_USDT"
	}
	return symbol
}

func (p *CryptoComProvider) convertInterval(interval string) string {
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
	return "5m"
}

func (p *CryptoComProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/public/get-candlestick?instrument_name=%s&timeframe=%s&count=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("crypto_com klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("crypto_com klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("crypto_com klines read failed: %w", err)
	}

	var result struct {
		Code   int `json:"code"`
		Result struct {
			Data []struct {
				O string `json:"o"` // open
				H string `json:"h"` // high
				L string `json:"l"` // low
				C string `json:"c"` // close
				V string `json:"v"` // volume
				T int64  `json:"t"` // timestamp in milliseconds
			} `json:"data"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("crypto_com klines parse failed: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("crypto_com API error: code=%d", result.Code)
	}

	klines := make([]Kline, 0, len(result.Result.Data))
	for _, item := range result.Result.Data {
		openTime := item.T
		open, _ := strconv.ParseFloat(item.O, 64)
		high, _ := strconv.ParseFloat(item.H, 64)
		low, _ := strconv.ParseFloat(item.L, 64)
		close, _ := strconv.ParseFloat(item.C, 64)
		volume, _ := strconv.ParseFloat(item.V, 64)

		intervalSeconds := getCryptoComIntervalSeconds(interval)
		closeTime := openTime + (intervalSeconds * 1000)

		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}

func (p *CryptoComProvider) GetOpenInterest(symbol string) (*OIData, error) {
	return nil, fmt.Errorf("Crypto.com is spot-only; open interest not available")
}

func (p *CryptoComProvider) GetFundingRate(symbol string) (float64, error) {
	return 0, fmt.Errorf("Crypto.com is spot-only; funding rate not available")
}

func getCryptoComIntervalSeconds(interval string) int64 {
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
	return 300
}

// KrakenProvider implements MarketDataProvider for Kraken exchange
type KrakenProvider struct {
baseURL string
}

func NewKrakenProvider() *KrakenProvider {
	return &KrakenProvider{
		baseURL: "https://api.kraken.com/0/public",
	}
}

func (p *KrakenProvider) GetName() string {
	return "kraken"
}

func (p *KrakenProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	// Kraken uses XXBTZUSD for BTC/USD
	if strings.Contains(symbol, "BTC") || strings.Contains(symbol, "XBT") {
		return "XXBTZUSD" // Standard Kraken BTC/USD pair
	}
	// For other symbols, convert to Kraken format
	symbol = strings.ReplaceAll(symbol, "BTC", "XBT")
	symbol = strings.ReplaceAll(symbol, "USDT", "")
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	if !strings.Contains(symbol, "Z") && !strings.Contains(symbol, "/") {
		return symbol + "ZUSD"
	}
	return symbol
}

func (p *KrakenProvider) convertInterval(interval string) string {
	intervalMap := map[string]int{
		"1m":  1,
		"3m":  3,
		"5m":  5,
		"15m": 15,
		"30m": 30,
		"1h":  60,
		"4h":  240,
		"1d":  1440,
	}
	if converted, ok := intervalMap[interval]; ok {
		return fmt.Sprintf("%d", converted)
	}
	return "5"
}

func (p *KrakenProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	intervalMinutes := p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/OHLC?pair=%s&interval=%s",
		p.baseURL, url.QueryEscape(symbol), intervalMinutes)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("kraken klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("kraken klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kraken klines read failed: %w", err)
	}

	var rawResponse struct {
		Error  []string          `json:"error"`
		Result json.RawMessage   `json:"result"`
	}

	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, fmt.Errorf("kraken klines parse failed: %w", err)
	}

	if len(rawResponse.Error) > 0 && rawResponse.Error[0] != "" {
		return nil, fmt.Errorf("kraken API error: %v", rawResponse.Error)
	}

	// Unmarshal result as a map to handle different pair names
	var resultMap map[string]json.RawMessage
	if err := json.Unmarshal(rawResponse.Result, &resultMap); err != nil {
		return nil, fmt.Errorf("kraken result parse failed: %w", err)
	}

	if len(resultMap) == 0 {
		return nil, fmt.Errorf("kraken: no data in result")
	}

	// Get the klines data - skip "last" key which is just a timestamp
	var klinesData [][]interface{}
	for pairName, pairData := range resultMap {
		// Skip "last" key - it's just a timestamp, not klines data
		if pairName == "last" {
			continue
		}
		
		// Try to unmarshal as array of arrays (the actual klines data)
		if err := json.Unmarshal(pairData, &klinesData); err != nil {
			return nil, fmt.Errorf("kraken klines data parse failed for pair %s: %w", pairName, err)
		}
		
		// Found valid klines data, break
		break
	}

	if len(klinesData) == 0 {
		return nil, fmt.Errorf("kraken: no klines data found (only 'last' timestamp present)")
	}

	// Take last 'limit' items
	startIdx := 0
	if len(klinesData) > limit {
		startIdx = len(klinesData) - limit
	}
	recentData := klinesData[startIdx:]

	klines := make([]Kline, 0, len(recentData))
	intervalMins, _ := strconv.Atoi(intervalMinutes)
	for _, item := range recentData {
		if len(item) < 8 {
			continue
		}
		// Kraken format: [time, open, high, low, close, vwap, volume, count]
		var openTime int64
		if t, ok := item[0].(float64); ok {
			openTime = int64(t) * 1000
		} else if t, ok := item[0].(int64); ok {
			openTime = t * 1000
		}
		// Parse as string first, then convert (Kraken returns strings)
		openStr := fmt.Sprintf("%v", item[1])
		highStr := fmt.Sprintf("%v", item[2])
		lowStr := fmt.Sprintf("%v", item[3])
		closeStr := fmt.Sprintf("%v", item[4])
		volumeStr := fmt.Sprintf("%v", item[6])
		
		open, _ := strconv.ParseFloat(openStr, 64)
		high, _ := strconv.ParseFloat(highStr, 64)
		low, _ := strconv.ParseFloat(lowStr, 64)
		close, _ := strconv.ParseFloat(closeStr, 64)
		volume, _ := strconv.ParseFloat(volumeStr, 64)

		closeTime := openTime + (int64(intervalMins) * 60 * 1000)

		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}
// KrakenProvider GetKlines, GetOpenInterest, GetFundingRate
func (p *KrakenProvider) GetOpenInterest(symbol string) (*OIData, error) {
	return nil, fmt.Errorf("Kraken is spot-only; open interest not available")
}

func (p *KrakenProvider) GetFundingRate(symbol string) (float64, error) {
	return 0, fmt.Errorf("Kraken is spot-only; funding rate not available")
}

// GeminiProvider implements MarketDataProvider for Gemini exchange
type GeminiProvider struct {
	baseURL string
}

func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{
		baseURL: "https://api.gemini.com/v2",
	}
}

func (p *GeminiProvider) GetName() string {
	return "gemini"
}

func (p *GeminiProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "USDT", "USD")
	symbol = strings.ReplaceAll(symbol, "_", "")
	return symbol
}

func (p *GeminiProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "1m",
		"3m":  "3m",
		"5m":  "5m",
		"15m": "15m",
		"30m": "30m",
		"1h":  "1hr",
		"4h":  "4hr",
		"1d":  "1day",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "5m"
}

func (p *GeminiProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/candles/%s/%s?limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("gemini klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini klines read failed: %w", err)
	}

	var rawData [][]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("gemini klines parse failed: %w", err)
	}

	klines := make([]Kline, len(rawData))
	for i, item := range rawData {
		if len(item) < 6 {
			continue
		}
		// Gemini format: [time, open, high, low, close, volume]
		openTime := int64(item[0].(float64))
		open, _ := parseFloat(item[1])
		high, _ := parseFloat(item[2])
		low, _ := parseFloat(item[3])
		close, _ := parseFloat(item[4])
		volume, _ := parseFloat(item[5])

		intervalSeconds := getGeminiIntervalSeconds(interval)
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

	return klines, nil
}

func (p *GeminiProvider) GetOpenInterest(symbol string) (*OIData, error) {
	return nil, fmt.Errorf("Gemini is spot-only; open interest not available")
}

func (p *GeminiProvider) GetFundingRate(symbol string) (float64, error) {
	return 0, fmt.Errorf("Gemini is spot-only; funding rate not available")
}

func getGeminiIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"1m":   60,
		"3m":   180,
		"5m":   300,
		"15m":  900,
		"30m":  1800,
		"1hr":  3600,
		"4hr":  14400,
		"1day": 86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 300
}

// DigifinexProvider implements MarketDataProvider for Digifinex exchange
type DigifinexProvider struct {
	baseURL string
}

func NewDigifinexProvider() *DigifinexProvider {
	return &DigifinexProvider{
		baseURL: "https://openapi.digifinex.com/v3",
	}
}

func (p *DigifinexProvider) GetName() string {
	return "digifinex"
}

func (p *DigifinexProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "-", "_")
	if !strings.Contains(symbol, "_") && strings.HasSuffix(symbol, "USDT") {
		base := symbol[:len(symbol)-4]
		return base + "_USDT"
	}
	return symbol
}

func (p *DigifinexProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "1",
		"3m":  "3",
		"5m":  "5",
		"15m": "15",
		"30m": "30",
		"1h":  "60",
		"4h":  "240",
		"1d":  "1D",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "5"
}

func (p *DigifinexProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	interval = p.convertInterval(interval)
	// Digifinex uses /kline (singular) not /klines
	apiURL := fmt.Sprintf("%s/kline?symbol=%s&period=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), interval, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("digifinex klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("digifinex klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("digifinex klines read failed: %w", err)
	}

	var result struct {
		Code int             `json:"code"`
		Data [][]interface{} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("digifinex klines parse failed: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("digifinex API error: code=%d", result.Code)
	}

	klines := make([]Kline, 0, len(result.Data))
	for _, item := range result.Data {
		if len(item) < 6 {
			continue
		}
		// Digifinex format: [timestamp (seconds), volume, high, low, open, close]
		openTimeSeconds := int64(item[0].(float64))
		openTime := openTimeSeconds * 1000 // Convert to milliseconds
		volume, _ := parseFloat(item[1])
		high, _ := parseFloat(item[2])
		low, _ := parseFloat(item[3])
		open, _ := parseFloat(item[4])
		close, _ := parseFloat(item[5])

		intervalSeconds := getDigifinexIntervalSeconds(interval)
		closeTime := openTime + (intervalSeconds * 1000)

		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}

func (p *DigifinexProvider) GetOpenInterest(symbol string) (*OIData, error) {
	return nil, fmt.Errorf("Digifinex is spot-only; open interest not available")
}

func (p *DigifinexProvider) GetFundingRate(symbol string) (float64, error) {
	return 0, fmt.Errorf("Digifinex is spot-only; funding rate not available")
}

func getDigifinexIntervalSeconds(interval string) int64 {
	intervalSecondsMap := map[string]int64{
		"1":   60,
		"3":   180,
		"5":   300,
		"15":  900,
		"30":  1800,
		"60":  3600,
		"240": 14400,
		"1D":  86400,
	}
	if seconds, ok := intervalSecondsMap[interval]; ok {
		return seconds
	}
	return 300
}

// WhitebitProvider implements MarketDataProvider for WhiteBIT exchange
type WhitebitProvider struct {
	baseURL string
}

func NewWhitebitProvider() *WhitebitProvider {
	return &WhitebitProvider{
		baseURL: "https://whitebit.com/api/v1/public",
	}
}

func (p *WhitebitProvider) GetName() string {
	return "whitebit"
}

func (p *WhitebitProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "-", "_")
	if !strings.Contains(symbol, "_") && strings.HasSuffix(symbol, "USDT") {
		base := symbol[:len(symbol)-4]
		return base + "_USDT"
	}
	return symbol
}

func (p *WhitebitProvider) convertInterval(interval string) string {
	// WhiteBIT supports: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M
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
	return "5m"
}

func (p *WhitebitProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	intervalStr := p.convertInterval(interval)
	// WhiteBIT API: /api/v1/public/kline?market=BTC_USDT&interval=5m&limit=2
	apiURL := fmt.Sprintf("%s/kline?market=%s&interval=%s&limit=%d",
		p.baseURL, url.QueryEscape(symbol), intervalStr, limit)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("whitebit klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("whitebit klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("whitebit klines read failed: %w", err)
	}

	var result struct {
		Success bool                    `json:"success"`
		Result  [][]interface{} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("whitebit klines parse failed: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("whitebit API error: success=false")
	}

	klines := make([]Kline, len(result.Result))
	for i, item := range result.Result {
		if len(item) < 6 {
			continue
		}
		// WhiteBIT format: [time (seconds), open, close, high, low, volume stock, volume money]
		openTimeSeconds := int64(item[0].(float64))
		openTime := openTimeSeconds * 1000 // Convert to milliseconds
		openStr := fmt.Sprintf("%v", item[1])
		closeStr := fmt.Sprintf("%v", item[2])
		highStr := fmt.Sprintf("%v", item[3])
		lowStr := fmt.Sprintf("%v", item[4])
		volumeStr := fmt.Sprintf("%v", item[5]) // Use volume stock
		
		open, _ := strconv.ParseFloat(openStr, 64)
		close, _ := strconv.ParseFloat(closeStr, 64)
		high, _ := strconv.ParseFloat(highStr, 64)
		low, _ := strconv.ParseFloat(lowStr, 64)
		volume, _ := strconv.ParseFloat(volumeStr, 64)

		// Calculate close time from interval
		intervalMinutes := getWhitebitIntervalMinutes(intervalStr)
		closeTime := openTime + (intervalMinutes * 60 * 1000)

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

	return klines, nil
}

func (p *WhitebitProvider) GetOpenInterest(symbol string) (*OIData, error) {
	return nil, fmt.Errorf("WhiteBIT is spot-only; open interest not available")
}

func (p *WhitebitProvider) GetFundingRate(symbol string) (float64, error) {
	return 0, fmt.Errorf("WhiteBIT is spot-only; funding rate not available")
}

func getWhitebitIntervalMinutes(interval string) int64 {
	intervalMap := map[string]int64{
		"1m":  1,
		"3m":  3,
		"5m":  5,
		"15m": 15,
		"30m": 30,
		"1h":  60,
		"4h":  240,
		"1d":  1440,
	}
	if minutes, ok := intervalMap[interval]; ok {
		return minutes
	}
	return 5
}

// UpbitProvider implements MarketDataProvider for Upbit exchange
type UpbitProvider struct {
	baseURL string
}

func NewUpbitProvider() *UpbitProvider {
	return &UpbitProvider{
		baseURL: "https://api.upbit.com/v1",
	}
}

func (p *UpbitProvider) GetName() string {
	return "upbit"
}

func (p *UpbitProvider) getUpbitSymbol(baseSymbol string) string {
	// Upbit uses KRW pairs
	symbol := strings.ToUpper(baseSymbol)
	symbol = strings.ReplaceAll(symbol, "USDT", "")
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	return "KRW-" + symbol
}

func (p *UpbitProvider) NormalizeSymbol(symbol string) string {
	return p.getUpbitSymbol(symbol)
}

func (p *UpbitProvider) convertInterval(interval string) string {
	intervalMap := map[string]string{
		"1m":  "1",
		"3m":  "3",
		"5m":  "5",
		"15m": "15",
		"30m": "30",
		"1h":  "60",
		"4h":  "240",
		"1d":  "240",
	}
	if converted, ok := intervalMap[interval]; ok {
		return converted
	}
	return "5"
}

func (p *UpbitProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	intervalMinutes := p.convertInterval(interval)
	apiURL := fmt.Sprintf("%s/candles/minutes/%s?market=%s&count=%d",
		p.baseURL, intervalMinutes, url.QueryEscape(symbol), limit)

	// For daily, use different endpoint
	if interval == "1d" {
		apiURL = fmt.Sprintf("%s/candles/days?market=%s&count=%d",
			p.baseURL, url.QueryEscape(symbol), limit)
	}

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("upbit klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("upbit klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("upbit klines read failed: %w", err)
	}

	var rawData []map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("upbit klines parse failed: %w", err)
	}

	klines := make([]Kline, len(rawData))
	// Upbit returns in reverse chronological order, reverse it
	for i := len(rawData) - 1; i >= 0; i-- {
		item := rawData[i]
		candleDateTimeKST := item["candle_date_time_kst"].(string)
		t, _ := time.Parse("2006-01-02T15:04:05", candleDateTimeKST)
		openTime := t.UnixMilli()
		open := parseFloatSafe(item["opening_price"])
		high := parseFloatSafe(item["high_price"])
		low := parseFloatSafe(item["low_price"])
		close := parseFloatSafe(item["trade_price"])
		volume := parseFloatSafe(item["candle_acc_trade_volume"])

		intervalSeconds := getUpbitIntervalSeconds(interval)
		closeTime := openTime + (intervalSeconds * 1000)

		klines[len(rawData)-1-i] = Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		}
	}

	return klines, nil
}

func (p *UpbitProvider) GetOpenInterest(symbol string) (*OIData, error) {
	return nil, fmt.Errorf("Upbit is spot-only; open interest not available")
}

func (p *UpbitProvider) GetFundingRate(symbol string) (float64, error) {
	return 0, fmt.Errorf("Upbit is spot-only; funding rate not available")
}

func getUpbitIntervalSeconds(interval string) int64 {
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
	return 300
}

// AlpacaCryptoProvider implements MarketDataProvider for Alpaca Crypto API
type AlpacaCryptoProvider struct {
	client *marketdata.Client
}

func NewAlpacaCryptoProvider() *AlpacaCryptoProvider {
	// Initialize Alpaca client (API keys optional for market data)
	// If not set, will use unauthenticated requests
	client := marketdata.NewClient(marketdata.ClientOpts{})
	
	return &AlpacaCryptoProvider{
		client: client,
	}
}

func (p *AlpacaCryptoProvider) GetName() string {
	return "alpaca_crypto"
}

// NormalizeSymbol converts NOFX symbol format to Alpaca format
// NOFX format: BTCUSDT -> Alpaca format: BTC/USD
func (p *AlpacaCryptoProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	
	// Remove underscores and hyphens
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	
	// Convert to Alpaca format: BTCUSDT -> BTC/USD
	// Alpaca uses BASE/QUOTE format, typically with USD as quote
	if strings.HasSuffix(symbol, "USDT") && len(symbol) > 4 {
		base := symbol[:len(symbol)-4]
		return base + "/USD"
	}
	if strings.HasSuffix(symbol, "USDC") && len(symbol) > 4 {
		base := symbol[:len(symbol)-4]
		return base + "/USD"
	}
	// If already in Alpaca format (contains /), return as is
	if strings.Contains(symbol, "/") {
		return symbol
	}
	
	// Default: assume USD pair
	if len(symbol) > 0 {
		return symbol + "/USD"
	}
	
	return symbol
}

// convertInterval converts standard interval to Alpaca TimeFrame
func (p *AlpacaCryptoProvider) convertInterval(interval string) (marketdata.TimeFrame, error) {
	intervalMap := map[string]marketdata.TimeFrame{
		"1m":  marketdata.OneMin,
		"3m":  marketdata.NewTimeFrame(3, marketdata.Min),
		"5m":  marketdata.NewTimeFrame(5, marketdata.Min),
		"15m": marketdata.NewTimeFrame(15, marketdata.Min),
		"30m": marketdata.NewTimeFrame(30, marketdata.Min),
		"1h":  marketdata.NewTimeFrame(1, marketdata.Hour),
		"4h":  marketdata.NewTimeFrame(4, marketdata.Hour),
		"1d":  marketdata.NewTimeFrame(1, marketdata.Day),
	}
	
	if tf, ok := intervalMap[strings.ToLower(interval)]; ok {
		return tf, nil
	}
	
	// Default to 1 minute
	return marketdata.OneMin, fmt.Errorf("unsupported interval %s, defaulting to 1m", interval)
}

// GetKlines fetches candlestick data from Alpaca Crypto API
func (p *AlpacaCryptoProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	alpacaSymbol := p.NormalizeSymbol(symbol)
	
	// Convert interval to Alpaca TimeFrame
	timeFrame, err := p.convertInterval(interval)
	if err != nil {
		// Use default if conversion fails
		timeFrame = marketdata.OneMin
	}
	
	// Calculate time range based on limit and interval
	// For Alpaca, we need to provide start and end times
	// Estimate time range: limit * interval duration
	endTime := time.Now()
	
	// Estimate start time based on interval and limit
	var duration time.Duration
	switch strings.ToLower(interval) {
	case "1m":
		duration = time.Duration(limit) * time.Minute
	case "3m":
		duration = time.Duration(limit*3) * time.Minute
	case "5m":
		duration = time.Duration(limit*5) * time.Minute
	case "15m":
		duration = time.Duration(limit*15) * time.Minute
	case "30m":
		duration = time.Duration(limit*30) * time.Minute
	case "1h":
		duration = time.Duration(limit) * time.Hour
	case "4h":
		duration = time.Duration(limit*4) * time.Hour
	case "1d":
		duration = time.Duration(limit) * 24 * time.Hour
	default:
		duration = time.Duration(limit) * time.Minute // Default to 1 minute
	}
	
	startTime := endTime.Add(-duration)
	
	// Add some buffer to ensure we get enough data
	startTime = startTime.Add(-time.Hour) // Add 1 hour buffer
	
	// Fetch historical bars from Alpaca
	alpacaBars, err := marketdata.GetCryptoBars(alpacaSymbol, marketdata.GetCryptoBarsRequest{
		TimeFrame:  timeFrame,
		Start:      startTime,
		End:        endTime,
		TotalLimit: limit,
		CryptoFeed: marketdata.US,
	})
	if err != nil {
		return nil, fmt.Errorf("alpaca crypto klines request failed: %w", err)
	}
	
	// Convert Alpaca bars to NOFX Kline format
	klines := make([]Kline, 0, len(alpacaBars))
	for _, bar := range alpacaBars {
		// Alpaca bars are already sorted by time
		openTime := bar.Timestamp.Unix() * 1000 // Convert to milliseconds
		closeTime := openTime + (60 * 1000)      // Approximate close time (1 minute later)
		
		// For longer intervals, adjust close time
		switch strings.ToLower(interval) {
		case "3m":
			closeTime = openTime + (3 * 60 * 1000)
		case "5m":
			closeTime = openTime + (5 * 60 * 1000)
		case "15m":
			closeTime = openTime + (15 * 60 * 1000)
		case "30m":
			closeTime = openTime + (30 * 60 * 1000)
		case "1h":
			closeTime = openTime + (60 * 60 * 1000)
		case "4h":
			closeTime = openTime + (4 * 60 * 60 * 1000)
		case "1d":
			closeTime = openTime + (24 * 60 * 60 * 1000)
		}
		
		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			CloseTime: closeTime,
		})
	}
	
	// Limit to requested number (Alpaca might return more due to buffer)
	if len(klines) > limit {
		klines = klines[len(klines)-limit:]
	}
	
	return klines, nil
}

func (p *AlpacaCryptoProvider) GetOpenInterest(symbol string) (*OIData, error) {
	// Alpaca Crypto API doesn't provide open interest data
	return &OIData{
		Latest:  0,
		Average: 0,
	}, fmt.Errorf("alpaca crypto does not support open interest")
}

func (p *AlpacaCryptoProvider) GetFundingRate(symbol string) (float64, error) {
	// Alpaca Crypto API doesn't provide funding rate data
	return 0, fmt.Errorf("alpaca crypto does not support funding rate")
}

