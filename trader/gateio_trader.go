package trader

import (
    "crypto/hmac"
    "crypto/sha512"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "math"
    "net/http"
    "net/url"
    "regexp"
    "strconv"
    "strings"
    "sync"
    "time"
)

// ContractInfo holds precision/contract metadata used for quantity formatting
type ContractInfo struct {
    QuantoMultiplier float64 `json:"quanto_multiplier"`
    OrderSizeMin     float64 `json:"order_size_min"`
    OrderPriceMin    float64 `json:"order_price_min"`
    TickSize         float64 `json:"tick_size"` // Price tick size for precision
}

// GateioTrader Gate.io交易器实现（HTTP 客户端 + 简单缓存）
type GateioTrader struct {
    apiKey    string
    secretKey string
    testnet   bool
    baseURL   string
    client    *http.Client

    // Cache
    cachedBalance     map[string]interface{}
    balanceCacheTime  time.Time
    balanceCacheMutex sync.RWMutex

    cachedPositions     []map[string]interface{}
    positionsCacheTime  time.Time
    positionsCacheMutex sync.RWMutex

    cacheDuration time.Duration

    // Contract precision cache
    contractPrecision map[string]ContractInfo
    precisionMutex    sync.RWMutex

    // Symbol conversion cache
    symbolCache struct {
        toGateio   map[string]string // BTCUSDT -> BTC_USDT
        fromGateio map[string]string // BTC_USDT -> BTCUSDT
        mu         sync.RWMutex
    }
}

// NewGateioTrader 创建Gate.io交易器
func NewGateioTrader(apiKey, secretKey string, testnet bool) (*GateioTrader, error) {
    baseURL := "https://api.gateio.ws/api/v4"
    if testnet {
        // Gate.io testnet uses different base URL
        baseURL = "https://api-testnet.gateapi.io/api/v4"
        log.Printf("✓ Gate.io 测试网模式已启用 (BaseURL: %s)", baseURL)
    } else {
        log.Printf("✓ Gate.io 主网模式 (BaseURL: %s)", baseURL)
    }

    t := &GateioTrader{
        apiKey:            apiKey,
        secretKey:         secretKey,
        testnet:           testnet,
        baseURL:           baseURL,
        client:            &http.Client{Timeout: 30 * time.Second},
        cacheDuration:     15 * time.Second,
        contractPrecision: make(map[string]ContractInfo),
    }
    t.symbolCache.toGateio = make(map[string]string)
    t.symbolCache.fromGateio = make(map[string]string)
    return t, nil
}

// --- Helpers ---

func (t *GateioTrader) signRequest(method, path, query, body string, timestamp string) string {
    // Gate.io API v4 signature format:
    // HMAC-SHA512(METHOD\nPREFIX+PATH\nQUERY\nBODY_HASH\nTIMESTAMP, secret_key)
    // Where:
    // - PREFIX is "/api/v4"
    // - PATH is the API endpoint path (e.g., "/futures/usdt/accounts")
    // - BODY_HASH is SHA512 hash of the request body (empty string -> empty hash)
    
    // Calculate SHA512 hash of body
    bodyHash := sha512.Sum512([]byte(body))
    bodyHashHex := hex.EncodeToString(bodyHash[:])
    
    // Build full path including prefix for signature
    fullPath := "/api/v4" + path
    
    // Build signature string: METHOD\nPREFIX+PATH\nQUERY\nBODY_HASH\nTIMESTAMP
    signatureString := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", 
        method, fullPath, query, bodyHashHex, timestamp)
    
    // Calculate HMAC-SHA512 signature
    mac := hmac.New(sha512.New, []byte(t.secretKey))
    mac.Write([]byte(signatureString))
    signature := hex.EncodeToString(mac.Sum(nil))
    
    return signature
}

// doRequest sends an authenticated HTTP request to Gate.io
func (t *GateioTrader) doRequest(method, path string, query url.Values, body string) ([]byte, error) {
    endpoint := t.baseURL + path
    q := ""
    if query != nil {
        q = query.Encode()
        if q != "" {
            endpoint += "?" + q
        }
    }

    req, err := http.NewRequest(method, endpoint, strings.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("创建请求失败: %w", err)
    }
    timestamp := fmt.Sprintf("%d", time.Now().Unix())
    signature := t.signRequest(method, path, q, body, timestamp)
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("KEY", t.apiKey)
    req.Header.Set("Timestamp", timestamp)
    req.Header.Set("SIGN", signature)
    
    // Debug logging (remove in production)
    if t.testnet {
        bodyHash := sha512.Sum512([]byte(body))
        bodyHashHex := hex.EncodeToString(bodyHash[:])
        fullPath := "/api/v4" + path
        log.Printf("🔍 Gate.io Debug: method=%s, fullPath=%s, query='%s', bodyHash=%s, timestamp=%s, sig=%s...", 
            method, fullPath, q, bodyHashHex[:16], timestamp, signature[:16])
    }

    resp, err := t.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("发送请求失败: %w", err)
    }
    defer resp.Body.Close()

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("读取响应失败: %w", err)
    }
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("Gate.io API返回错误 (status %d): %s", resp.StatusCode, string(data))
    }
    return data, nil
}

// convertSymbolToGateio converts internal symbol format to Gate.io format
// Examples: BTCUSDT -> BTC_USDT
func (t *GateioTrader) convertSymbolToGateio(symbol string) string {
    t.symbolCache.mu.RLock()
    if v, ok := t.symbolCache.toGateio[symbol]; ok {
        t.symbolCache.mu.RUnlock()
        return v
    }
    t.symbolCache.mu.RUnlock()

    re := regexp.MustCompile(`^([A-Z]+)(USDT|USDC|BTC|ETH|BUSD)$`)
    if m := re.FindStringSubmatch(symbol); len(m) == 3 {
        converted := fmt.Sprintf("%s_%s", m[1], m[2])
        t.symbolCache.mu.Lock()
        t.symbolCache.toGateio[symbol] = converted
        t.symbolCache.mu.Unlock()
        return converted
    }
    if strings.Contains(symbol, "_") {
        return symbol
    }
    log.Printf("⚠️  Gate.io符号未能转换: %s", symbol)
    return symbol
}

// convertSymbolFromGateio converts Gate.io format back to internal format
func (t *GateioTrader) convertSymbolFromGateio(gateioSymbol string) string {
    t.symbolCache.mu.RLock()
    if v, ok := t.symbolCache.fromGateio[gateioSymbol]; ok {
        t.symbolCache.mu.RUnlock()
        return v
    }
    t.symbolCache.mu.RUnlock()
    converted := strings.ReplaceAll(gateioSymbol, "_", "")
    t.symbolCache.mu.Lock()
    t.symbolCache.fromGateio[gateioSymbol] = converted
    t.symbolCache.mu.Unlock()
    return converted
}

// --- Trader interface stubs (to be completed) ---

func (t *GateioTrader) GetBalance() (map[string]interface{}, error) {
    // GET /futures/usdt/accounts with caching
    t.balanceCacheMutex.RLock()
    if time.Since(t.balanceCacheTime) < t.cacheDuration && t.cachedBalance != nil {
        defer t.balanceCacheMutex.RUnlock()
        return t.cachedBalance, nil
    }
    t.balanceCacheMutex.RUnlock()

    data, err := t.doRequest("GET", "/futures/usdt/accounts", nil, "")
    if err != nil {
        return nil, err
    }
    
    // Debug: Log raw response for testnet
    if t.testnet {
        log.Printf("🔍 Gate.io Balance Raw Response: %s", string(data))
    }
    
    // Gate.io may return numeric values as strings, so parse flexibly
    var acc map[string]interface{}
    if err := json.Unmarshal(data, &acc); err != nil {
        return nil, fmt.Errorf("解析账户响应失败: %w", err)
    }
    
    // Debug: Log parsed values
    if t.testnet {
        log.Printf("🔍 Gate.io Parsed Balance: total=%v, available=%v, unrealised_pnl=%v", 
            acc["total"], acc["available"], acc["unrealised_pnl"])
    }
    
    // Helper to parse float from string or number
    parseFloat := func(v interface{}) float64 {
        switch val := v.(type) {
        case float64:
            return val
        case float32:
            return float64(val)
        case int:
            return float64(val)
        case int64:
            return float64(val)
        case string:
            f, err := strconv.ParseFloat(val, 64)
            if err != nil {
                return 0
            }
            return f
        default:
            return 0
        }
    }
    
    // Match the field names expected by auto_trader.go
    // Gate.io "total" = wallet balance (cross_margin_balance or total without unrealized PnL)
    // Gate.io "unrealised_pnl" = unrealized profit/loss
    // Gate.io "available" = available balance
    totalWalletBalance := parseFloat(acc["total"]) - parseFloat(acc["unrealised_pnl"])
    totalUnrealizedPnL := parseFloat(acc["unrealised_pnl"])
    availableBalance := parseFloat(acc["available"])
    
    resp := map[string]interface{}{
        "totalWalletBalance":    totalWalletBalance,  // Wallet balance without unrealized PnL
        "totalUnrealizedProfit": totalUnrealizedPnL,  // Unrealized PnL
        "availableBalance":      availableBalance,     // Available balance
        // Also include original fields for debugging
        "total_equity":          parseFloat(acc["total"]),
        "total_unrealized_pnl":  totalUnrealizedPnL,
    }
    t.balanceCacheMutex.Lock()
    t.cachedBalance = resp
    t.balanceCacheTime = time.Now()
    t.balanceCacheMutex.Unlock()
    return resp, nil
}

func (t *GateioTrader) GetPositions() ([]map[string]interface{}, error) {
    // GET /futures/usdt/positions with caching and symbol conversion
    t.positionsCacheMutex.RLock()
    if time.Since(t.positionsCacheTime) < t.cacheDuration && t.cachedPositions != nil {
        defer t.positionsCacheMutex.RUnlock()
        return t.cachedPositions, nil
    }
    t.positionsCacheMutex.RUnlock()

    data, err := t.doRequest("GET", "/futures/usdt/positions", nil, "")
    if err != nil {
        return nil, err
    }
    
    // Gate.io returns numeric values as strings, parse flexibly
    var raw []map[string]interface{}
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, fmt.Errorf("解析持仓响应失败: %w", err)
    }
    
    // Helper to parse float from string or number
    parseFloat := func(v interface{}) float64 {
        switch val := v.(type) {
        case float64:
            return val
        case float32:
            return float64(val)
        case int:
            return float64(val)
        case int64:
            return float64(val)
        case string:
            f, err := strconv.ParseFloat(val, 64)
            if err != nil {
                return 0
            }
            return f
        default:
            return 0
        }
    }
    
    positions := make([]map[string]interface{}, 0, len(raw))
    for _, p := range raw {
        size := parseFloat(p["size"])
        if size == 0 {
            continue
        }
        
        contract, _ := p["contract"].(string)
        leverage := parseFloat(p["leverage"])
        entryPrice := parseFloat(p["entry_price"])
        value := parseFloat(p["value"])  // Position value in USDT (negative = short, positive = long)
        
        // Gate.io: negative size = short, positive size = long
        // Keep value to calculate notional, but use size sign for side
        side := "long"
        if size < 0 {
            side = "short"
            size = -size  // Make size positive
        }
        
        // Calculate mark price from value and size
        markPrice := 0.0
        if size != 0 {
            markPrice = math.Abs(value) / size
        }
        
        unrealizedPnl := parseFloat(p["unrealised_pnl"])
        
        positions = append(positions, map[string]interface{}{
            "symbol":             t.convertSymbolFromGateio(contract),
            "positionAmt":        size,
            "entryPrice":         entryPrice,
            "markPrice":          markPrice,
            "leverage":           leverage,
            "unRealizedProfit":   unrealizedPnl,
            "liquidationPrice":   parseFloat(p["liq_price"]),
            "side":               side,
        })
    }
    t.positionsCacheMutex.Lock()
    t.cachedPositions = positions
    t.positionsCacheTime = time.Now()
    t.positionsCacheMutex.Unlock()
    return positions, nil
}

func (t *GateioTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
    // Cancel existing orders first
    if err := t.CancelAllOrders(symbol); err != nil {
        log.Printf("  ⚠ 取消旧订单失败: %v", err)
    }

    // Set leverage
    if err := t.SetLeverage(symbol, leverage); err != nil {
        return nil, fmt.Errorf("设置杠杆失败: %w", err)
    }

    gateSymbol := t.convertSymbolToGateio(symbol)

    // Get contract info first to convert quantity properly
    contractInfo, err := t.getContractInfo(symbol)
    if err != nil {
        return nil, fmt.Errorf("获取合约信息失败: %w", err)
    }

    // Get market price for limit order (use slightly higher price to ensure execution)
    price, err := t.GetMarketPrice(symbol)
    if err != nil {
        return nil, fmt.Errorf("获取市场价格失败: %w", err)
    }

    // Use aggressive limit price (1% above market for long)
    limitPrice := price * 1.01
    
    // Format price according to contract's tick size
    priceStr, err := t.FormatPrice(symbol, limitPrice)
    if err != nil {
        return nil, fmt.Errorf("格式化价格失败: %w", err)
    }

    // Gate.io order: size is positive for long, negative for short
    // For quanto contracts, size needs to be in contracts (integer)
    // Convert base quantity to contracts using quanto_multiplier
    var sizeInContracts int64
    if contractInfo.QuantoMultiplier > 0 {
        contractsFloat := quantity / contractInfo.QuantoMultiplier
        sizeInContracts = int64(contractsFloat + 0.5) // Round to nearest integer
    } else {
        // Fallback: use quantity directly as integer
        sizeInContracts = int64(quantity + 0.5)
    }
    
    // Ensure minimum size (OrderSizeMin is already in contracts)
    minContracts := int64(contractInfo.OrderSizeMin)
    if sizeInContracts < minContracts {
        sizeInContracts = minContracts
    }

    // Gate.io order: size is positive for long, negative for short
    // Use IOC (Immediate or Cancel) order type for market-like execution
    orderBody := map[string]interface{}{
        "contract": gateSymbol,
        "size":     sizeInContracts, // Positive for long, integer (contracts)
        "price":    priceStr,        // String: formatted price
        "tif":      "ioc",           // Immediate or Cancel (market-like)
        "text":     fmt.Sprintf("t-%s", symbol), // Client order ID
        "reduce_only": false,        // Not reducing existing position
    }

    bodyJSON, err := json.Marshal(orderBody)
    if err != nil {
        return nil, fmt.Errorf("序列化订单失败: %w", err)
    }

    data, err := t.doRequest("POST", "/futures/usdt/orders", nil, string(bodyJSON))
    if err != nil {
        return nil, fmt.Errorf("开多仓失败: %w", err)
    }

    var result map[string]interface{}
    if err := json.Unmarshal(data, &result); err != nil {
        return nil, fmt.Errorf("解析订单响应失败: %w", err)
    }

    log.Printf("✓ 开多仓成功: %s 数量: %d contracts", symbol, sizeInContracts)

    // Invalidate position cache
    t.positionsCacheMutex.Lock()
    t.positionsCacheTime = time.Time{}
    t.positionsCacheMutex.Unlock()

    return result, nil
}

func (t *GateioTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
    // Cancel existing orders first
    if err := t.CancelAllOrders(symbol); err != nil {
        log.Printf("  ⚠ 取消旧订单失败: %v", err)
    }

    // Set leverage
    if err := t.SetLeverage(symbol, leverage); err != nil {
        return nil, fmt.Errorf("设置杠杆失败: %w", err)
    }

    gateSymbol := t.convertSymbolToGateio(symbol)

    // Get contract info first to convert quantity properly
    contractInfo, err := t.getContractInfo(symbol)
    if err != nil {
        return nil, fmt.Errorf("获取合约信息失败: %w", err)
    }

    // Get market price for limit order (use slightly lower price to ensure execution)
    price, err := t.GetMarketPrice(symbol)
    if err != nil {
        return nil, fmt.Errorf("获取市场价格失败: %w", err)
    }

    // Use aggressive limit price (1% below market for short)
    limitPrice := price * 0.99
    
    // Format price according to contract's tick size
    priceStr, err := t.FormatPrice(symbol, limitPrice)
    if err != nil {
        return nil, fmt.Errorf("格式化价格失败: %w", err)
    }

    // Gate.io order: size is negative for short
    // For quanto contracts, size needs to be in contracts (integer)
    // Convert base quantity to contracts using quanto_multiplier
    // contracts = base_quantity / quanto_multiplier
    var sizeInContracts int64
    if contractInfo.QuantoMultiplier > 0 {
        contractsFloat := quantity / contractInfo.QuantoMultiplier
        sizeInContracts = int64(contractsFloat + 0.5) // Round to nearest integer
    } else {
        // Fallback: use quantity directly as integer
        sizeInContracts = int64(quantity + 0.5)
    }
    
    // Ensure minimum size (OrderSizeMin is already in contracts)
    minContracts := int64(contractInfo.OrderSizeMin)
    if sizeInContracts < minContracts {
        sizeInContracts = minContracts
    }
    
    // Make negative for short
    sizeInContracts = -sizeInContracts

    // Use IOC (Immediate or Cancel) order type for market-like execution
    // Gate.io API: size is INTEGER (contracts), price is STRING
    orderBody := map[string]interface{}{
        "contract": gateSymbol,
        "size":     sizeInContracts, // Integer: negative for short, positive for long
        "price":    priceStr,        // String: formatted price
        "tif":      "ioc",           // Immediate or Cancel (market-like)
        "text":     fmt.Sprintf("t-%s", symbol), // Client order ID
        "reduce_only": false,        // Not reducing existing position
    }

    bodyJSON, err := json.Marshal(orderBody)
    if err != nil {
        return nil, fmt.Errorf("序列化订单失败: %w", err)
    }
    
    // Debug: Log the order body for testnet
    if t.testnet {
        log.Printf("🔍 Gate.io Order Debug: Order body: %s", string(bodyJSON))
        // Also log contract info for debugging
        info, infoErr := t.getContractInfo(symbol)
        if infoErr == nil {
            log.Printf("🔍 Gate.io Order Debug: Contract tick_size=%v, order_price_min=%v", info.TickSize, info.OrderPriceMin)
        }
    }

    data, err := t.doRequest("POST", "/futures/usdt/orders", nil, string(bodyJSON))
    if err != nil {
        return nil, fmt.Errorf("开空仓失败: %w", err)
    }

    var result map[string]interface{}
    if err := json.Unmarshal(data, &result); err != nil {
        return nil, fmt.Errorf("解析订单响应失败: %w", err)
    }

    log.Printf("✓ 开空仓成功: %s 数量: %d contracts", symbol, -sizeInContracts)

    // Invalidate position cache
    t.positionsCacheMutex.Lock()
    t.positionsCacheTime = time.Time{}
    t.positionsCacheMutex.Unlock()

    return result, nil
}

func (t *GateioTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
    gateSymbol := t.convertSymbolToGateio(symbol)

    // Fetch position directly from Gate.io API to get exact size in contracts
    // This avoids conversion issues and ensures we use the exact position size
    data, err := t.doRequest("GET", "/futures/usdt/positions", nil, "")
    if err != nil {
        return nil, fmt.Errorf("获取持仓失败: %w", err)
    }

    var raw []map[string]interface{}
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, fmt.Errorf("解析持仓响应失败: %w", err)
    }

    // Helper to parse float from string or number
    parseFloat := func(v interface{}) float64 {
        switch val := v.(type) {
        case float64:
            return val
        case float32:
            return float64(val)
        case int:
            return float64(val)
        case int64:
            return float64(val)
        case string:
            f, err := strconv.ParseFloat(val, 64)
            if err != nil {
                return 0
            }
            return f
        default:
            return 0
        }
    }

    // Find the position for this symbol
    var positionSize float64
    var positionValue float64
    found := false
    
    for _, p := range raw {
        contract, _ := p["contract"].(string)
        if contract != gateSymbol {
            continue
        }
        
        size := parseFloat(p["size"])
        value := parseFloat(p["value"])
        
        // Check if it's a long position (size > 0 for long in Gate.io)
        if size > 0 {
            positionSize = size  // Size is already in contracts from Gate.io
            positionValue = value
            found = true
            break
        }
    }

    if !found || positionSize == 0 {
        return nil, fmt.Errorf("没有找到 %s 的多仓", symbol)
    }

    // Use quantity parameter if provided, otherwise use position size
    var sizeInContracts int64
    if quantity > 0 {
        // If quantity is provided, treat it as base quantity and convert to contracts
        contractInfo, err := t.getContractInfo(symbol)
        if err != nil {
            return nil, fmt.Errorf("获取合约信息失败: %w", err)
        }
        
        if contractInfo.QuantoMultiplier > 0 {
            contractsFloat := quantity / contractInfo.QuantoMultiplier
            sizeInContracts = int64(contractsFloat + 0.5)
        } else {
            sizeInContracts = int64(quantity + 0.5)
        }
        
        // Ensure we don't exceed the actual position size
        maxContracts := int64(positionSize)
        if sizeInContracts > maxContracts {
            sizeInContracts = maxContracts
        }
    } else {
        // Use exact position size from Gate.io (already in contracts)
        sizeInContracts = int64(positionSize)
    }

    log.Printf("  📊 获取到多仓数量: %d contracts (value: %.2f USDT)", sizeInContracts, positionValue)

    // Get market price for limit order (use slightly lower price to ensure execution)
    price, err := t.GetMarketPrice(symbol)
    if err != nil {
        return nil, fmt.Errorf("获取市场价格失败: %w", err)
    }

    // Use aggressive limit price (1% below market to close long)
    limitPrice := price * 0.99
    
    // Format price according to contract's tick size
    priceStr, err := t.FormatPrice(symbol, limitPrice)
    if err != nil {
        return nil, fmt.Errorf("格式化价格失败: %w", err)
    }

    // Gate.io: to close long, use negative size with reduce_only
    sizeInContracts = -sizeInContracts

    orderBody := map[string]interface{}{
        "contract":    gateSymbol,
        "size":        sizeInContracts, // Negative to close long, integer (contracts)
        "price":       priceStr,
        "tif":         "ioc",
        "text":        fmt.Sprintf("t-%s", symbol), // Client order ID
        "reduce_only": true, // Important: reduce only to close position
    }

    bodyJSON, err := json.Marshal(orderBody)
    if err != nil {
        return nil, fmt.Errorf("序列化订单失败: %w", err)
    }

    data, err = t.doRequest("POST", "/futures/usdt/orders", nil, string(bodyJSON))
    if err != nil {
        return nil, fmt.Errorf("平多仓失败: %w", err)
    }

    var result map[string]interface{}
    if err := json.Unmarshal(data, &result); err != nil {
        return nil, fmt.Errorf("解析订单响应失败: %w", err)
    }

    log.Printf("✓ 平多仓成功: %s 数量: %d contracts", symbol, -sizeInContracts)

    // Invalidate position cache
    t.positionsCacheMutex.Lock()
    t.positionsCacheTime = time.Time{}
    t.positionsCacheMutex.Unlock()

    return result, nil
}

func (t *GateioTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
    gateSymbol := t.convertSymbolToGateio(symbol)

    // Fetch position directly from Gate.io API to get exact size in contracts
    // This avoids conversion issues and ensures we use the exact position size
    data, err := t.doRequest("GET", "/futures/usdt/positions", nil, "")
    if err != nil {
        return nil, fmt.Errorf("获取持仓失败: %w", err)
    }

    var raw []map[string]interface{}
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, fmt.Errorf("解析持仓响应失败: %w", err)
    }

    // Helper to parse float from string or number
    parseFloat := func(v interface{}) float64 {
        switch val := v.(type) {
        case float64:
            return val
        case float32:
            return float64(val)
        case int:
            return float64(val)
        case int64:
            return float64(val)
        case string:
            f, err := strconv.ParseFloat(val, 64)
            if err != nil {
                return 0
            }
            return f
        default:
            return 0
        }
    }

    // Find the position for this symbol
    var positionSize float64
    var positionValue float64
    found := false
    
    // Debug: log all positions to understand the data structure
    log.Printf("  🔍 CloseShort调试: 查找 %s (Gate.io格式: %s)", symbol, gateSymbol)
    log.Printf("  🔍 CloseShort调试: 持仓列表中有 %d 个持仓", len(raw))
    
    for i, p := range raw {
        contract, _ := p["contract"].(string)
        size := parseFloat(p["size"])
        value := parseFloat(p["value"])
        
        // Debug: log each position
        if size != 0 {
            log.Printf("  🔍 CloseShort调试: 持仓[%d] contract=%s, size=%.8f, value=%.8f", i, contract, size, value)
        }
        
        if contract != gateSymbol {
            continue
        }
        
        log.Printf("  🔍 CloseShort调试: 找到匹配的合约 %s, size=%.8f, value=%.8f", contract, size, value)
        
        // Check if it's a short position (size < 0 for short in Gate.io)
        if size < 0 {
            positionSize = -size  // Make size positive (already in contracts from Gate.io)
            positionValue = math.Abs(value)
            found = true
            log.Printf("  ✓ CloseShort调试: 确认空仓 size=%.8f (原始%.8f), value=%.8f", positionSize, size, positionValue)
            break
        } else if size > 0 {
            log.Printf("  ⚠️ CloseShort调试: 合约 %s 是多仓 (size > 0), 不是空仓", contract)
        }
    }

    if !found || positionSize == 0 {
        log.Printf("  ❌ CloseShort调试: 未找到空仓 - found=%v, positionSize=%.8f", found, positionSize)
        return nil, fmt.Errorf("没有找到 %s 的空仓", symbol)
    }

    // Use quantity parameter if provided, otherwise use position size
    var sizeInContracts int64
    if quantity > 0 {
        // If quantity is provided, treat it as base quantity and convert to contracts
        contractInfo, err := t.getContractInfo(symbol)
        if err != nil {
            return nil, fmt.Errorf("获取合约信息失败: %w", err)
        }
        
        if contractInfo.QuantoMultiplier > 0 {
            contractsFloat := quantity / contractInfo.QuantoMultiplier
            sizeInContracts = int64(contractsFloat + 0.5)
        } else {
            sizeInContracts = int64(quantity + 0.5)
        }
        
        // Ensure we don't exceed the actual position size
        maxContracts := int64(positionSize)
        if sizeInContracts > maxContracts {
            sizeInContracts = maxContracts
        }
    } else {
        // Use exact position size from Gate.io (already in contracts)
        sizeInContracts = int64(positionSize)
    }

    log.Printf("  📊 获取到空仓数量: %d contracts (value: %.2f USDT)", sizeInContracts, positionValue)

    // Get market price for limit order (use slightly higher price to ensure execution)
    price, err := t.GetMarketPrice(symbol)
    if err != nil {
        return nil, fmt.Errorf("获取市场价格失败: %w", err)
    }

    // Use aggressive limit price (1% above market to close short)
    limitPrice := price * 1.01
    
    // Format price according to contract's tick size
    priceStr, err := t.FormatPrice(symbol, limitPrice)
    if err != nil {
        return nil, fmt.Errorf("格式化价格失败: %w", err)
    }

    // Gate.io: to close short, use positive size with reduce_only
    orderBody := map[string]interface{}{
        "contract":    gateSymbol,
        "size":        sizeInContracts, // Positive to close short, integer (contracts)
        "price":       priceStr,
        "tif":         "ioc",
        "text":        fmt.Sprintf("t-%s", symbol), // Client order ID
        "reduce_only": true, // Important: reduce only to close position
    }

    bodyJSON, err := json.Marshal(orderBody)
    if err != nil {
        return nil, fmt.Errorf("序列化订单失败: %w", err)
    }

    data, err = t.doRequest("POST", "/futures/usdt/orders", nil, string(bodyJSON))
    if err != nil {
        return nil, fmt.Errorf("平空仓失败: %w", err)
    }

    var result map[string]interface{}
    if err := json.Unmarshal(data, &result); err != nil {
        return nil, fmt.Errorf("解析订单响应失败: %w", err)
    }

    log.Printf("✓ 平空仓成功: %s 数量: %d contracts", symbol, sizeInContracts)

    // Invalidate position cache
    t.positionsCacheMutex.Lock()
    t.positionsCacheTime = time.Time{}
    t.positionsCacheMutex.Unlock()

    return result, nil
}

func (t *GateioTrader) CancelAllOrders(symbol string) error {
    gateSymbol := t.convertSymbolToGateio(symbol)
    query := url.Values{}
    query.Set("contract", gateSymbol)
    query.Set("settle", "usdt")

    _, err := t.doRequest("DELETE", "/futures/usdt/orders", query, "")
    if err != nil {
        // Don't fail hard if there are no orders to cancel
        log.Printf("⚠️  取消订单失败 (可能没有订单): %v", err)
        return nil
    }

    log.Printf("✓ 已取消 %s 的所有订单", symbol)
    return nil
}

func (t *GateioTrader) SetLeverage(symbol string, leverage int) error {
    gateSymbol := t.convertSymbolToGateio(symbol)
    
    // Gate.io requires leverage as query parameter (not in body) based on API docs
    query := url.Values{}
    query.Set("leverage", fmt.Sprintf("%d", leverage))

    // Debug: log the request
    if t.testnet {
        log.Printf("🔍 Gate.io SetLeverage Debug: symbol=%s, gateSymbol=%s, query=%s", symbol, gateSymbol, query.Encode())
    }

    _, err := t.doRequest("POST", fmt.Sprintf("/futures/usdt/positions/%s/leverage", gateSymbol), query, "")
    if err != nil {
        return fmt.Errorf("设置杠杆失败: %w", err)
    }

    log.Printf("✓ 设置杠杆成功: %s = %dx", symbol, leverage)
    return nil
}

// getContractInfo fetches contract information including precision and min order size
func (t *GateioTrader) getContractInfo(symbol string) (*ContractInfo, error) {
    t.precisionMutex.RLock()
    if info, ok := t.contractPrecision[symbol]; ok {
        t.precisionMutex.RUnlock()
        return &info, nil
    }
    t.precisionMutex.RUnlock()

    gateSymbol := t.convertSymbolToGateio(symbol)
    data, err := t.doRequest("GET", fmt.Sprintf("/futures/usdt/contracts/%s", gateSymbol), nil, "")
    if err != nil {
        return nil, fmt.Errorf("获取合约信息失败: %w", err)
    }

    var contract map[string]interface{}
    if err := json.Unmarshal(data, &contract); err != nil {
        return nil, fmt.Errorf("解析合约信息失败: %w", err)
    }

    // Debug: Log raw contract data for testnet
    if t.testnet {
        contractJSON, _ := json.MarshalIndent(contract, "", "  ")
        log.Printf("🔍 Gate.io Contract Debug: Raw contract data:\n%s", string(contractJSON))
    }

    // Helper to parse float from string or number
    parseFloat := func(v interface{}) float64 {
        switch val := v.(type) {
        case float64:
            return val
        case float32:
            return float64(val)
        case int:
            return float64(val)
        case int64:
            return float64(val)
        case string:
            f, err := strconv.ParseFloat(val, 64)
            if err != nil {
                return 0
            }
            return f
        default:
            return 0
        }
    }

    // Gate.io uses "order_price_round" as the tick size for order prices
    // This is the precision that prices must follow
    tickSize := parseFloat(contract["order_price_round"])
    if tickSize == 0 {
        // Fallback: try other possible field names
        tickSize = parseFloat(contract["tick_size"])
        if tickSize == 0 {
            tickSize = parseFloat(contract["order_price_tick"])
            if tickSize == 0 {
                tickSize = parseFloat(contract["price_tick"])
            }
        }
    }
    
    orderPriceMin := parseFloat(contract["order_price_min"])
    if orderPriceMin == 0 {
        orderPriceMin = parseFloat(contract["price_min"])
    }

    info := ContractInfo{
        QuantoMultiplier: parseFloat(contract["quanto_multiplier"]),
        OrderSizeMin:     parseFloat(contract["order_size_min"]),
        OrderPriceMin:    orderPriceMin,
        TickSize:         tickSize,
    }
    
    // Debug: Log parsed values
    if t.testnet {
        log.Printf("🔍 Gate.io Contract Debug: Parsed - quanto_multiplier=%v, order_size_min=%v, order_price_min=%v, tick_size=%v",
            info.QuantoMultiplier, info.OrderSizeMin, info.OrderPriceMin, info.TickSize)
    }

    t.precisionMutex.Lock()
    t.contractPrecision[symbol] = info
    t.precisionMutex.Unlock()

    return &info, nil
}

func (t *GateioTrader) GetMarketPrice(symbol string) (float64, error) {
    gateSymbol := t.convertSymbolToGateio(symbol)
    query := url.Values{}
    query.Set("contract", gateSymbol)
    
    data, err := t.doRequest("GET", "/futures/usdt/tickers", query, "")
    if err != nil {
        return 0, fmt.Errorf("获取市场价格失败: %w", err)
    }

    var tickers []map[string]interface{}
    if err := json.Unmarshal(data, &tickers); err != nil {
        return 0, fmt.Errorf("解析价格响应失败: %w", err)
    }

    if len(tickers) == 0 {
        return 0, fmt.Errorf("未找到合约 %s 的价格数据", symbol)
    }

    // Helper to parse float from string or number
    parseFloat := func(v interface{}) float64 {
        switch val := v.(type) {
        case float64:
            return val
        case float32:
            return float64(val)
        case int:
            return float64(val)
        case int64:
            return float64(val)
        case string:
            f, err := strconv.ParseFloat(val, 64)
            if err != nil {
                return 0
            }
            return f
        default:
            return 0
        }
    }

    // Use last price, fallback to mark_price
    price := parseFloat(tickers[0]["last"])
    if price == 0 {
        price = parseFloat(tickers[0]["mark_price"])
    }
    if price == 0 {
        return 0, fmt.Errorf("无法获取有效价格")
    }

    return price, nil
}

func (t *GateioTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
    gateSymbol := t.convertSymbolToGateio(symbol)

    // Get contract info to convert quantity to contracts
    contractInfo, err := t.getContractInfo(symbol)
    if err != nil {
        return fmt.Errorf("获取合约信息失败: %w", err)
    }

    // Convert base quantity to contracts
    var sizeInContracts int64
    if contractInfo.QuantoMultiplier > 0 {
        contractsFloat := quantity / contractInfo.QuantoMultiplier
        sizeInContracts = int64(contractsFloat + 0.5)
    } else {
        sizeInContracts = int64(quantity + 0.5)
    }

    // Determine size direction based on position side
    if positionSide == "LONG" {
        sizeInContracts = -sizeInContracts // Negative to close long
    }
    // For SHORT, size stays positive

    // Format stop price according to contract's tick size
    priceStr, err := t.FormatPrice(symbol, stopPrice)
    if err != nil {
        return fmt.Errorf("格式化止损价失败: %w", err)
    }

    // Gate.io price_orders rule: 1 = >=, 2 = <=
    // For stop loss: LONG triggers when price goes DOWN (<=), SHORT triggers when price goes UP (>=)
    var rule int
    if positionSide == "LONG" {
        rule = 2 // LONG stop loss: trigger when price <= stopPrice
    } else {
        rule = 1 // SHORT stop loss: trigger when price >= stopPrice
    }

    priceOrderBody := map[string]interface{}{
        "initial": map[string]interface{}{
            "contract":     gateSymbol,
            "size":         sizeInContracts, // Use explicit size instead of auto_size
            "price":        "0", // Market price when triggered
            "tif":          "ioc",
            "text":         fmt.Sprintf("t-sl-%s", symbol),
            "reduce_only":  true, // Required for closing orders
        },
        "trigger": map[string]interface{}{
            "strategy_type": 0, // 0 = commutative
            "price_type":    0, // 0 = latest price
            "price":          priceStr,
            "rule":           rule,
            "expiration":     604800, // 7 days in seconds
        },
    }

    bodyJSON, err := json.Marshal(priceOrderBody)
    if err != nil {
        return fmt.Errorf("序列化止损订单失败: %w", err)
    }

    // Debug log
    if t.testnet {
        log.Printf("🔍 Gate.io SL Order Debug: Order body: %s", string(bodyJSON))
    }

    data, err := t.doRequest("POST", "/futures/usdt/price_orders", nil, string(bodyJSON))
    if err != nil {
        return fmt.Errorf("设置止损失败: %w", err)
    }

    var result map[string]interface{}
    if err := json.Unmarshal(data, &result); err != nil {
        return fmt.Errorf("解析止损订单响应失败: %w", err)
    }

    // Debug: log response for testnet
    if t.testnet {
        log.Printf("🔍 Gate.io SL Order Response: %s", string(data))
    }

    // Check for error in response body (Gate.io may return errors in JSON even with 200 status)
    // Gate.io error responses typically have "label" or "message" fields
    if errLabel, ok := result["label"].(string); ok && errLabel != "" {
        errMsg := errLabel
        if msg, ok := result["message"].(string); ok && msg != "" {
            errMsg = fmt.Sprintf("%s: %s", errLabel, msg)
        }
        return fmt.Errorf("设置止损失败: Gate.io返回错误: %s (完整响应: %s)", errMsg, string(data))
    }
    if errMsg, ok := result["message"].(string); ok && errMsg != "" && errMsg != "ok" {
        return fmt.Errorf("设置止损失败: Gate.io返回错误: %s (完整响应: %s)", errMsg, string(data))
    }

    // Check if order ID is present (success indicator)
    // If response is an array or has order info, that's also success
    if _, hasOrderID := result["id"]; !hasOrderID {
        // If no order ID and no error message, log warning but don't fail
        // (some Gate.io responses might not include ID)
        if t.testnet {
            log.Printf("⚠️  止损订单响应中未找到订单ID，但也没有错误信息 (响应: %s)", string(data))
        }
    }

    log.Printf("  止损价设置: %.4f", stopPrice)
    return nil
}

func (t *GateioTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
    gateSymbol := t.convertSymbolToGateio(symbol)

    // Get contract info to convert quantity to contracts
    contractInfo, err := t.getContractInfo(symbol)
    if err != nil {
        return fmt.Errorf("获取合约信息失败: %w", err)
    }

    // Convert base quantity to contracts
    var sizeInContracts int64
    if contractInfo.QuantoMultiplier > 0 {
        contractsFloat := quantity / contractInfo.QuantoMultiplier
        sizeInContracts = int64(contractsFloat + 0.5)
    } else {
        sizeInContracts = int64(quantity + 0.5)
    }

    // Determine size direction based on position side
    if positionSide == "LONG" {
        sizeInContracts = -sizeInContracts // Negative to close long
    }
    // For SHORT, size stays positive

    // Format take profit price according to contract's tick size
    priceStr, err := t.FormatPrice(symbol, takeProfitPrice)
    if err != nil {
        return fmt.Errorf("格式化止盈价失败: %w", err)
    }

    // Gate.io price_orders rule: 1 = >=, 2 = <=
    // For take profit: LONG triggers when price goes UP (>=), SHORT triggers when price goes DOWN (<=)
    var rule int
    if positionSide == "LONG" {
        rule = 1 // LONG take profit: trigger when price >= takeProfitPrice
    } else {
        rule = 2 // SHORT take profit: trigger when price <= takeProfitPrice
    }

    priceOrderBody := map[string]interface{}{
        "initial": map[string]interface{}{
            "contract":     gateSymbol,
            "size":         sizeInContracts, // Use explicit size instead of auto_size
            "price":        "0", // Market price when triggered
            "tif":          "ioc",
            "text":         fmt.Sprintf("t-tp-%s", symbol),
            "reduce_only":  true, // Required for closing orders
        },
        "trigger": map[string]interface{}{
            "strategy_type": 0, // 0 = commutative
            "price_type":    0, // 0 = latest price
            "price":          priceStr,
            "rule":           rule,
            "expiration":     604800, // 7 days in seconds
        },
    }

    bodyJSON, err := json.Marshal(priceOrderBody)
    if err != nil {
        return fmt.Errorf("序列化止盈订单失败: %w", err)
    }

    // Debug log
    if t.testnet {
        log.Printf("🔍 Gate.io TP Order Debug: Order body: %s", string(bodyJSON))
    }

    data, err := t.doRequest("POST", "/futures/usdt/price_orders", nil, string(bodyJSON))
    if err != nil {
        return fmt.Errorf("设置止盈失败: %w", err)
    }

    var result map[string]interface{}
    if err := json.Unmarshal(data, &result); err != nil {
        return fmt.Errorf("解析止盈订单响应失败: %w", err)
    }

    // Debug: log response for testnet
    if t.testnet {
        log.Printf("🔍 Gate.io TP Order Response: %s", string(data))
    }

    // Check for error in response body (Gate.io may return errors in JSON even with 200 status)
    // Gate.io error responses typically have "label" or "message" fields
    if errLabel, ok := result["label"].(string); ok && errLabel != "" {
        errMsg := errLabel
        if msg, ok := result["message"].(string); ok && msg != "" {
            errMsg = fmt.Sprintf("%s: %s", errLabel, msg)
        }
        return fmt.Errorf("设置止盈失败: Gate.io返回错误: %s (完整响应: %s)", errMsg, string(data))
    }
    if errMsg, ok := result["message"].(string); ok && errMsg != "" && errMsg != "ok" {
        return fmt.Errorf("设置止盈失败: Gate.io返回错误: %s (完整响应: %s)", errMsg, string(data))
    }

    // Check if order ID is present (success indicator)
    // If response is an array or has order info, that's also success
    if _, hasOrderID := result["id"]; !hasOrderID {
        // If no order ID and no error message, log warning but don't fail
        // (some Gate.io responses might not include ID)
        if t.testnet {
            log.Printf("⚠️  止盈订单响应中未找到订单ID，但也没有错误信息 (响应: %s)", string(data))
        }
    }

    log.Printf("  止盈价设置: %.4f", takeProfitPrice)
    return nil
}

func (t *GateioTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
    info, err := t.getContractInfo(symbol)
    if err != nil {
        // Fallback to 6 decimal places if we can't get contract info
        return fmt.Sprintf("%.6f", quantity), nil
    }

    // Gate.io uses quanto multiplier for size calculation
    // Order size should be integer (in contracts), but we're working with base quantity
    // For simplicity, format to 6 decimal places, but ensure it meets minimum
    if quantity < info.OrderSizeMin && info.OrderSizeMin > 0 {
        quantity = info.OrderSizeMin
    }

    // Format to reasonable precision (6 decimals)
    return fmt.Sprintf("%.6f", quantity), nil
}

// FormatPrice formats price according to contract's tick size
func (t *GateioTrader) FormatPrice(symbol string, price float64) (string, error) {
    info, err := t.getContractInfo(symbol)
    if err != nil {
        // Fallback to 2 decimal places if we can't get contract info
        return fmt.Sprintf("%.2f", price), nil
    }

    // If tick size is available, round to nearest tick
    if info.TickSize > 0 {
        // Round to nearest tick
        price = float64(int64(price/info.TickSize+0.5)) * info.TickSize
        
        // Calculate precision from tick size
        // e.g., tick_size = 0.1 -> 1 decimal, tick_size = 0.01 -> 2 decimals
        precision := 0
        tick := info.TickSize
        for tick < 1.0 && precision < 10 {
            tick *= 10
            precision++
        }
        
        return fmt.Sprintf("%.*f", precision, price), nil
    }

    // Fallback: use 2 decimal places, but ensure it meets minimum
    if price < info.OrderPriceMin && info.OrderPriceMin > 0 {
        price = info.OrderPriceMin
    }
    
    return fmt.Sprintf("%.2f", price), nil
}

// Utility to decode JSON for future use
func decodeJSON(data []byte, v interface{}) error {
    if err := json.Unmarshal(data, v); err != nil {
        return fmt.Errorf("JSON解析失败: %w", err)
    }
    return nil
}


