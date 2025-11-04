package manager

import (
	"fmt"
	"log"
	"nofx/config"
	"nofx/trader"
	"sync"
	"time"
)

// TraderManager 管理多个trader实例
type TraderManager struct {
    traders map[string]*trader.AutoTrader // key: trader ID
    mu      sync.RWMutex
}

// NewTraderManager 创建trader管理器
func NewTraderManager() *TraderManager {
	return &TraderManager{
		traders: make(map[string]*trader.AutoTrader),
	}
}

// AddTrader 添加一个trader
func (tm *TraderManager) AddTrader(cfg config.TraderConfig, coinPoolURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, leverage config.LeverageConfig, positionSize config.PositionSizeConfig) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.traders[cfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", cfg.ID)
	}

	// 构建AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    cfg.ID,
		Name:                  cfg.Name,
		AIModel:               cfg.AIModel,
		Exchange:              cfg.Exchange,
		BinanceAPIKey:         cfg.BinanceAPIKey,
		BinanceSecretKey:      cfg.BinanceSecretKey,
		BinanceTestnet:        cfg.BinanceTestnet,
		HyperliquidPrivateKey: cfg.HyperliquidPrivateKey,
		HyperliquidWalletAddr: cfg.HyperliquidWalletAddr,
		HyperliquidTestnet:    cfg.HyperliquidTestnet,
		AsterUser:             cfg.AsterUser,
		AsterSigner:           cfg.AsterSigner,
		AsterPrivateKey:       cfg.AsterPrivateKey,
		GateioAPIKey:          cfg.GateioAPIKey,
		GateioSecretKey:       cfg.GateioSecretKey,
		GateioTestnet:         cfg.GateioTestnet,
		CoinPoolAPIURL:        coinPoolURL,
		UseQwen:               cfg.AIModel == "qwen",
		DeepSeekKey:           cfg.DeepSeekKey,
		QwenKey:               cfg.QwenKey,
		CustomAPIURL:          cfg.CustomAPIURL,
		CustomAPIKey:          cfg.CustomAPIKey,
		CustomModelName:       cfg.CustomModelName,
		ScanInterval:          cfg.GetScanInterval(),
		InitialBalance:        cfg.InitialBalance,
		BTCETHLeverage:        leverage.BTCETHLeverage,  // 使用配置的杠杆倍数
		AltcoinLeverage:       leverage.AltcoinLeverage, // 使用配置的杠杆倍数
		MinPositionSizeUSD:    positionSize.MinPositionSizeUSD,
		MaxPositionSizeUSD:    positionSize.MaxPositionSizeUSD,
		MaxMarginUsagePct:     positionSize.MaxMarginUsagePct,
		MaxPositionSizeMult:   positionSize.MaxPositionSizeMult,
		SafetyBufferPct:       positionSize.SafetyBufferPct,
		CheckAvailableBeforeOpen: positionSize.CheckAvailableBeforeOpen,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		SystemPromptTemplate:  cfg.SystemPromptTemplate, // 系统提示词模板名称
	}

	// 创建trader实例
	at, err := trader.NewAutoTrader(traderConfig)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}

	tm.traders[cfg.ID] = at
	log.Printf("✓ Trader '%s' (%s) 已添加", cfg.Name, cfg.AIModel)
	return nil
}

// GetTrader 获取指定ID的trader
func (tm *TraderManager) GetTrader(id string) (*trader.AutoTrader, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.traders[id]
	if !exists {
		return nil, fmt.Errorf("trader ID '%s' 不存在", id)
	}
	return t, nil
}

// GetAllTraders 获取所有trader
func (tm *TraderManager) GetAllTraders() map[string]*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.AutoTrader)
	for id, t := range tm.traders {
		result[id] = t
	}
	return result
}

// GetTraderIDs 获取所有trader ID列表
func (tm *TraderManager) GetTraderIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := make([]string, 0, len(tm.traders))
	for id := range tm.traders {
		ids = append(ids, id)
	}
	return ids
}

// StartAll 启动所有trader
func (tm *TraderManager) StartAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("🚀 启动所有Trader...")
	for id, t := range tm.traders {
		go func(traderID string, at *trader.AutoTrader) {
			log.Printf("▶️  启动 %s...", at.GetName())
			if err := at.Run(); err != nil {
				log.Printf("❌ %s 运行错误: %v", at.GetName(), err)
			}
		}(id, t)
	}
}

// StopAll 停止所有trader
func (tm *TraderManager) StopAll() {
    tm.mu.RLock()
    defer tm.mu.RUnlock()

    log.Println("⏹  停止所有Trader...")
    for _, t := range tm.traders {
        t.Stop()
    }
}

// StartDecisionLogCleanup 启动决策日志清理定时任务（与机器人一起运行）
// 返回一个停止函数用于优雅关闭
func (tm *TraderManager) StartDecisionLogCleanup(retentionDays int, interval time.Duration) func() {
    stop := make(chan struct{})

    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        // 立即执行一次，以免等待首个tick
        tm.runDecisionLogCleanup(retentionDays)

        for {
            select {
            case <-ticker.C:
                tm.runDecisionLogCleanup(retentionDays)
            case <-stop:
                log.Println("🧹 决策日志清理任务已停止")
                return
            }
        }
    }()

    log.Printf("🧹 已启动决策日志清理任务：保留%d天，每%d小时执行一次", retentionDays, int(interval.Hours()))

    return func() { close(stop) }
}

// runDecisionLogCleanup 执行一次清理任务
func (tm *TraderManager) runDecisionLogCleanup(retentionDays int) {
    tm.mu.RLock()
    defer tm.mu.RUnlock()

    for _, at := range tm.traders {
        if at == nil {
            continue
        }
        dl := at.GetDecisionLogger()
        if dl == nil {
            continue
        }
        if err := dl.CleanOldRecords(retentionDays); err != nil {
            log.Printf("⚠️ 决策日志清理失败（%s）: %v", at.GetName(), err)
        }
    }
}

// GetComparisonData 获取对比数据
func (tm *TraderManager) GetComparisonData() (map[string]interface{}, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	comparison := make(map[string]interface{})
	traders := make([]map[string]interface{}, 0, len(tm.traders))

	for _, t := range tm.traders {
		account, err := t.GetAccountInfo()
		if err != nil {
			continue
		}

		status := t.GetStatus()

		traders = append(traders, map[string]interface{}{
			"trader_id":       t.GetID(),
			"trader_name":     t.GetName(),
			"ai_model":        t.GetAIModel(),
			"total_equity":    account["total_equity"],
			"total_pnl":       account["total_pnl"],
			"total_pnl_pct":   account["total_pnl_pct"],
			"position_count":  account["position_count"],
			"margin_used_pct": account["margin_used_pct"],
			"call_count":      status["call_count"],
			"is_running":      status["is_running"],
		})
	}

	comparison["traders"] = traders
	comparison["count"] = len(traders)

	return comparison, nil
}
