package main

import (
    "fmt"
    "log"
    "nofx/api"
    "nofx/config"
    "nofx/manager"
    "nofx/market"
    "nofx/pool"
    "os"
    "os/signal"
    "strconv"
    "strings"
    "syscall"
    "time"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║    🏆 AI模型交易竞赛系统 - Qwen vs DeepSeek               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 加载配置文件
	configFile := "config.json"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	log.Printf("📋 加载配置文件: %s", configFile)
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}

	log.Printf("✓ 配置加载成功，共%d个trader参赛", len(cfg.Traders))
	fmt.Println()

	// Check for PORT environment variable (required for Render, Heroku, etc.)
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		port, err := strconv.Atoi(portEnv)
		if err == nil {
			cfg.APIServerPort = port
			log.Printf("✓ 使用环境变量 PORT: %d", port)
		}
	}

	// 初始化市场数据提供者
	market.InitializeProviders()

	// 设置市场数据提供者
	providerName := cfg.MarketDataProvider
	if providerName == "" {
		providerName = "binance" // Default
	}
	if err := market.SetDefaultProviderName(providerName); err != nil {
		log.Printf("⚠️  设置市场数据提供者失败 (%s)，使用默认值 binance: %v", providerName, err)
		market.SetDefaultProviderName("binance")
	} else {
		log.Printf("✓ 市场数据源: %s", providerName)
	}

	// 设置默认主流币种列表
	pool.SetDefaultCoins(cfg.DefaultCoins)

	// 设置是否使用默认主流币种
	pool.SetUseDefaultCoins(cfg.UseDefaultCoins)
	if cfg.UseDefaultCoins {
		log.Printf("✓ 已启用默认主流币种列表（共%d个币种）: %v", len(cfg.DefaultCoins), cfg.DefaultCoins)
	}

	// 设置币种池API URL
	if cfg.CoinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(cfg.CoinPoolAPIURL)
		log.Printf("✓ 已配置AI500币种池API")
	}
	if cfg.OITopAPIURL != "" {
		pool.SetOITopAPI(cfg.OITopAPIURL)
		log.Printf("✓ 已配置OI Top API")
	}

	// 创建TraderManager
	traderManager := manager.NewTraderManager()

	// 添加所有启用的trader
	enabledCount := 0
	for i, traderCfg := range cfg.Traders {
		// 跳过未启用的trader
		if !traderCfg.Enabled {
			log.Printf("⏭️  [%d/%d] 跳过未启用的 %s", i+1, len(cfg.Traders), traderCfg.Name)
			continue
		}

		enabledCount++
		log.Printf("📦 [%d/%d] 初始化 %s (%s模型)...",
			i+1, len(cfg.Traders), traderCfg.Name, strings.ToUpper(traderCfg.AIModel))

		err := traderManager.AddTrader(
			traderCfg,
			cfg.CoinPoolAPIURL,
			cfg.MaxDailyLoss,
			cfg.MaxDrawdown,
			cfg.StopTradingMinutes,
			cfg.Leverage, // 传递杠杆配置
			cfg.PositionSize, // 传递仓位大小配置
		)
		if err != nil {
			log.Fatalf("❌ 初始化trader失败: %v", err)
		}
	}

	// 检查是否至少有一个启用的trader
	if enabledCount == 0 {
		log.Fatalf("❌ 没有启用的trader，请在config.json中设置至少一个trader的enabled=true")
	}

	fmt.Println()
	fmt.Println("🏁 竞赛参赛者:")
	for _, traderCfg := range cfg.Traders {
		// 只显示启用的trader
		if !traderCfg.Enabled {
			continue
		}
		fmt.Printf("  • %s (%s) - 初始资金: %.0f USDT\n",
			traderCfg.Name, strings.ToUpper(traderCfg.AIModel), traderCfg.InitialBalance)
	}

	fmt.Println()
	fmt.Println("🤖 AI全权决策模式:")
	fmt.Printf("  • AI将自主决定每笔交易的杠杆倍数（山寨币最高%d倍，BTC/ETH最高%d倍）\n",
		cfg.Leverage.AltcoinLeverage, cfg.Leverage.BTCETHLeverage)
	fmt.Println("  • AI将自主决定每笔交易的仓位大小")
	fmt.Println("  • AI将自主设置止损和止盈价格")
	fmt.Println("  • AI将基于市场数据、技术指标、账户状态做出全面分析")
	fmt.Println()
	fmt.Println("⚠️  风险提示: AI自动交易有风险，建议小额资金测试！")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止运行")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// 创建并启动API服务器
	apiServer := api.NewServer(traderManager, cfg.APIServerPort)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("❌ API服务器错误: %v", err)
		}
	}()

	// 设置优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    // 启动所有trader
    traderManager.StartAll()

    // 启动决策日志清理任务（与Bot同进程运行，适用于本地和Docker）
    stopCleanup := traderManager.StartDecisionLogCleanup(
        cfg.DecisionLogRetentionDays,
        time.Duration(cfg.DecisionLogCleanupIntervalHours)*time.Hour,
    )

	// 等待退出信号
	<-sigChan
    fmt.Println()
    fmt.Println()
    log.Println("📛 收到退出信号，正在停止所有trader...")
    // 停止清理任务
    stopCleanup()
    traderManager.StopAll()

	fmt.Println()
	fmt.Println("👋 感谢使用AI交易竞赛系统！")
}
