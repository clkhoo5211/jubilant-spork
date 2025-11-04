package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

// BinanceProvider implements MarketDataProvider for Binance exchange
type BinanceProvider struct {
	baseURL string // e.g., "https://fapi.binance.com" for futures
}

// NewBinanceProvider creates a new Binance provider (defaults to futures API)
func NewBinanceProvider() *BinanceProvider {
	return &BinanceProvider{
		baseURL: "https://fapi.binance.com",
	}
}

// GetName returns the provider name
func (p *BinanceProvider) GetName() string {
	return "binance"
}

// NormalizeSymbol converts symbol to Binance format (e.g., BTCUSDT -> BTCUSDT)
func (p *BinanceProvider) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	// Remove underscores and hyphens
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	return symbol
}

// GetKlines fetches candlestick data from Binance
func (p *BinanceProvider) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	symbol = p.NormalizeSymbol(symbol)
	url := fmt.Sprintf("%s/fapi/v1/klines?symbol=%s&interval=%s&limit=%d",
		p.baseURL, symbol, interval, limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("binance klines request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance klines API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("binance klines read failed: %w", err)
	}

	var rawData [][]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("binance klines parse failed: %w", err)
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

// GetOpenInterest fetches open interest data from Binance
func (p *BinanceProvider) GetOpenInterest(symbol string) (*OIData, error) {
	symbol = p.NormalizeSymbol(symbol)
	url := fmt.Sprintf("%s/fapi/v1/openInterest?symbol=%s", p.baseURL, symbol)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("binance open interest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance open interest API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("binance open interest read failed: %w", err)
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("binance open interest parse failed: %w", err)
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // Approximate average
	}, nil
}

// GetFundingRate fetches funding rate from Binance
func (p *BinanceProvider) GetFundingRate(symbol string) (float64, error) {
	symbol = p.NormalizeSymbol(symbol)
	url := fmt.Sprintf("%s/fapi/v1/premiumIndex?symbol=%s", p.baseURL, symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("binance funding rate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return 0, fmt.Errorf("binance funding rate API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("binance funding rate read failed: %w", err)
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("binance funding rate parse failed: %w", err)
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)
	return rate, nil
}

