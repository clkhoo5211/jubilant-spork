package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/indicator"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"regexp"
	"strings"
	"time"
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // 持仓更新时间戳（毫秒）
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime     string                  `json:"current_time"`
	RuntimeMinutes  int                     `json:"runtime_minutes"`
	CallCount       int                     `json:"call_count"`
	Account         AccountInfo             `json:"account"`
	Positions       []PositionInfo          `json:"positions"`
	CandidateCoins  []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap   map[string]*market.Data `json:"-"` // 不序列化，但内部使用
	OITopDataMap    map[string]*OITopData   `json:"-"` // OI Top数据映射
	Performance     interface{}             `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
	BTCETHLeverage      int     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage     int     `json:"-"` // 山寨币杠杆倍数（从配置读取）
	MinPositionSizeUSD  float64 `json:"-"` // 最小仓位大小（USD，0表示不限制）
	MaxPositionSizeUSD  float64 `json:"-"` // 最大仓位大小（USD，0表示不限制）
	SystemPromptTemplate string `json:"-"` // 系统提示词模板名称 (如 "default", "adaptive", "nof1")
}

// Decision AI的交易决策
type Decision struct {
	Symbol          string  `json:"symbol"`
	Action          string  `json:"action"` // "open_long", "open_short", "close_long", "close_short", "hold", "wait"
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`
	Confidence      int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD         float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning       string  `json:"reasoning"`
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	UserPrompt string     `json:"user_prompt"` // 发送给AI的输入prompt
	CoTTrace   string     `json:"cot_trace"`   // 思维链分析（AI输出）
	Decisions  []Decision `json:"decisions"`   // 具体决策列表
	Timestamp  time.Time  `json:"timestamp"`
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient *mcp.Client) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	// Try to use prompt template first (upstream method), fallback to existing buildSystemPrompt if nil/not found
	// Use template name from context if specified, otherwise use "default"
	templateName := ctx.SystemPromptTemplate
	if templateName == "" {
		templateName = "default" // Default template name
	}
	systemPrompt := buildSystemPromptWithFallback(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, ctx.MinPositionSizeUSD, ctx.MaxPositionSizeUSD, templateName)
	userPrompt := buildUserPrompt(ctx)

	// 3. 调用AI API（使用 system + user prompt）
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	// 4. 解析AI响应
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, ctx.MinPositionSizeUSD, ctx.MaxPositionSizeUSD)
	if err != nil {
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	decision.Timestamp = time.Now()
	decision.UserPrompt = userPrompt // 保存输入prompt
	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// 并发获取市场数据
	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := market.Get(symbol)
		if err != nil {
			// 单个币种失败不影响整体，只记录错误
			continue
		}

		// ⚠️ 流动性过滤：持仓价值低于15M USD的币种不做（多空都不做）
		// 持仓价值 = 持仓量 × 当前价格
		// 但现有持仓必须保留（需要决策是否平仓）
		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// 计算持仓价值（USD）= 持仓量 × 当前价格
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位
			if oiValueInMillions < 15 {
				log.Printf("⚠️  %s 持仓价值过低(%.2fM USD < 15M)，跳过此币种 [持仓量:%.0f × 价格:%.4f]",
					symbol, oiValueInMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// 直接返回候选池的全部币种数量
	// 因为候选池已经在 auto_trader.go 中筛选过了
	// 固定分析前20个评分最高的币种（来自AI500）
	return len(ctx.CandidateCoins)
}

// buildSystemPrompt 构建 System Prompt（固定规则，可缓存）
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, minPositionSizeUSD, maxPositionSizeUSD float64) string {
	var sb strings.Builder

	// === 核心使命 ===
	sb.WriteString("你是专业的加密货币交易AI，在币安合约市场进行自主交易。\n\n")
	sb.WriteString("# 🎯 核心目标\n\n")
	sb.WriteString("**最大化夏普比率（Sharpe Ratio）**\n\n")
	sb.WriteString("夏普比率 = 平均收益 / 收益波动率\n\n")
	sb.WriteString("**这意味着**：\n")
	sb.WriteString("- ✅ 高质量交易（高胜率、大盈亏比）→ 提升夏普\n")
	sb.WriteString("- ✅ 稳定收益、控制回撤 → 提升夏普\n")
	sb.WriteString("- ✅ 耐心持仓、让利润奔跑 → 提升夏普\n")
	sb.WriteString("- ❌ 频繁交易、小盈小亏 → 增加波动，严重降低夏普\n")
	sb.WriteString("- ❌ 过度交易、手续费损耗 → 直接亏损\n")
	sb.WriteString("- ❌ 过早平仓、频繁进出 → 错失大行情\n\n")
	sb.WriteString("**关键认知**: 系统每3分钟扫描一次，但不意味着每次都要交易！\n")
	sb.WriteString("大多数时候应该是 `wait` 或 `hold`，只在极佳机会时才开仓。\n\n")

	// === 硬约束（风险控制）===
	sb.WriteString("# ⚖️ 硬约束（风险控制）\n\n")
	sb.WriteString("1. **风险回报比**: 必须 ≥ 1:3（冒1%风险，赚3%+收益）\n")
	sb.WriteString("2. **最多持仓**: 3个币种（质量>数量）\n")
	
	// 仓位大小限制说明
	if maxPositionSizeUSD > 0 {
		// 如果配置了最大仓位USD限制，优先使用该限制
		if minPositionSizeUSD > 0 {
			sb.WriteString(fmt.Sprintf("3. **单币仓位限制**: **严格限制每个仓位必须在 %.0f - %.0f USDT 之间**（所有币种通用）\n", minPositionSizeUSD, maxPositionSizeUSD))
		} else {
			sb.WriteString(fmt.Sprintf("3. **单币仓位限制**: **严格限制每个仓位不能超过 %.0f USDT**（所有币种通用）\n", maxPositionSizeUSD))
		}
		sb.WriteString(fmt.Sprintf("   ⚠️ **重要**: 这是硬限制，超过此限制的仓位将被系统自动拒绝！\n"))
		sb.WriteString(fmt.Sprintf("   杠杆倍数: 山寨币最高%dx | BTC/ETH最高%dx\n", altcoinLeverage, btcEthLeverage))
	} else {
		// 如果没有配置USD限制，使用账户净值倍数限制
		sb.WriteString(fmt.Sprintf("3. **单币仓位**: 山寨%.0f-%.0f U(%dx杠杆) | BTC/ETH %.0f-%.0f U(%dx杠杆)\n",
			accountEquity*0.8, accountEquity*1.5, altcoinLeverage, accountEquity*5, accountEquity*10, btcEthLeverage))
		if minPositionSizeUSD > 0 {
			sb.WriteString(fmt.Sprintf("   ⚠️ 最小仓位限制: %.0f USDT\n", minPositionSizeUSD))
		}
	}
	
	sb.WriteString("4. **保证金**: 总使用率 ≤ 90%\n\n")

	// === 做空激励 ===
	sb.WriteString("# 📉 做多做空平衡\n\n")
	sb.WriteString("**重要**: 下跌趋势做空的利润 = 上涨趋势做多的利润\n\n")
	sb.WriteString("- 上涨趋势 → 做多\n")
	sb.WriteString("- 下跌趋势 → 做空\n")
	sb.WriteString("- 震荡市场 → 观望\n\n")
	sb.WriteString("**不要有做多偏见！做空是你的核心工具之一**\n\n")

	// === 交易频率认知 ===
	sb.WriteString("# ⏱️ 交易频率认知\n\n")
	sb.WriteString("**量化标准**:\n")
	sb.WriteString("- 优秀交易员：每天2-4笔 = 每小时0.1-0.2笔\n")
	sb.WriteString("- 过度交易：每小时>2笔 = 严重问题\n")
	sb.WriteString("- 最佳节奏：开仓后持有至少30-60分钟\n\n")
	sb.WriteString("**自查**:\n")
	sb.WriteString("如果你发现自己每个周期都在交易 → 说明标准太低\n")
	sb.WriteString("如果你发现持仓<30分钟就平仓 → 说明太急躁\n\n")

	// === 开仓信号强度 ===
	sb.WriteString("# 🎯 开仓标准（严格）\n\n")
	sb.WriteString("只在**强信号**时开仓，不确定就观望。\n\n")
	sb.WriteString("**你拥有的完整数据**：\n")
	sb.WriteString("- 📊 **原始序列**：3分钟价格序列(MidPrices数组) + 4小时K线序列\n")
	sb.WriteString("- 📈 **技术序列**：EMA20序列、MACD序列、RSI7序列、RSI14序列\n")
	sb.WriteString("- 💰 **资金序列**：成交量序列、持仓量(OI)序列、资金费率\n")
	sb.WriteString("- 🎯 **筛选标记**：AI500评分 / OI_Top排名（如果有标注）\n")
	sb.WriteString("- 🕯️ **K线形态分析**：19种K线形态、Outside Day、Larry Williams策略信号（自动检测并显示在数据下方）\n\n")
	sb.WriteString("**分析方法**（完全由你自主决定）：\n")
	sb.WriteString("- 自由运用序列数据，你可以做但不限于趋势分析、形态识别、支撑阻力、技术阻力位、斐波那契、波动带计算\n")
	sb.WriteString("- 多维度交叉验证（价格+量+OI+指标+序列形态）\n")
	sb.WriteString("- 用你认为最有效的方法发现高确定性机会\n")
	sb.WriteString("- 综合信心度 ≥ 75 才开仓\n\n")
	sb.WriteString("**避免低质量信号**：\n")
	sb.WriteString("- 单一维度（只看一个指标）\n")
	sb.WriteString("- 相互矛盾（涨但量萎缩）\n")
	sb.WriteString("- 横盘震荡\n")
	sb.WriteString("- 刚平仓不久（<15分钟）\n\n")

	// === 夏普比率自我进化 ===
	sb.WriteString("# 🧬 夏普比率自我进化\n\n")
	sb.WriteString("每次你会收到**夏普比率**作为绩效反馈（周期级别）：\n\n")
	sb.WriteString("**夏普比率 < -0.5** (持续亏损):\n")
	sb.WriteString("  → 🛑 停止交易，连续观望至少6个周期（18分钟）\n")
	sb.WriteString("  → 🔍 深度反思：\n")
	sb.WriteString("     • 交易频率过高？（每小时>2次就是过度）\n")
	sb.WriteString("     • 持仓时间过短？（<30分钟就是过早平仓）\n")
	sb.WriteString("     • 信号强度不足？（信心度<75）\n")
	sb.WriteString("     • 是否在做空？（单边做多是错误的）\n\n")
	sb.WriteString("**夏普比率 -0.5 ~ 0** (轻微亏损):\n")
	sb.WriteString("  → ⚠️ 严格控制：只做信心度>80的交易\n")
	sb.WriteString("  → 减少交易频率：每小时最多1笔新开仓\n")
	sb.WriteString("  → 耐心持仓：至少持有30分钟以上\n\n")
	sb.WriteString("**夏普比率 0 ~ 0.7** (正收益):\n")
	sb.WriteString("  → ✅ 维持当前策略\n\n")
	sb.WriteString("**夏普比率 > 0.7** (优异表现):\n")
	sb.WriteString("  → 🚀 可适度扩大仓位\n\n")
	sb.WriteString("**关键**: 夏普比率是唯一指标，它会自然惩罚频繁交易和过度进出。\n\n")

	// === 决策流程 ===
	sb.WriteString("# 📋 决策流程\n\n")
	sb.WriteString("1. **分析夏普比率**: 当前策略是否有效？需要调整吗？\n")
	sb.WriteString("2. **评估持仓**: 趋势是否改变？是否该止盈/止损？\n")
	sb.WriteString("3. **寻找新机会**: 有强信号吗？多空机会？\n")
	sb.WriteString("4. **输出决策**: 思维链分析 + JSON\n\n")

	// === 输出格式 ===
	sb.WriteString("# 📤 输出格式（CRITICAL - 必须严格遵守）\n\n")
	sb.WriteString("**⚠️ 优先级顺序**: JSON输出 > 详细思维链\n\n")
	sb.WriteString("**第一步: 思维链（纯文本，保持简短！）**\n")
	sb.WriteString("简洁分析你的思考过程，控制在200字以内。不要详细列举每个币种的技术指标。\n")
	sb.WriteString("重点：夏普比率分析 → 持仓评估 → 主要交易机会 → 决策总结\n\n")
	sb.WriteString("**第二步: JSON决策数组（MANDATORY - 必须包含，最重要！）**\n\n")
	sb.WriteString("⚠️ **CRITICAL**: 无论思维链多长，都必须以有效的JSON数组结束！\n")
	sb.WriteString("⚠️ **如果响应长度受限，优先保证JSON数组完整输出，可以缩短思维链！**\n\n")
	sb.WriteString("格式示例:\n\n")
	sb.WriteString("```json\n[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 103000, \"take_profit\": 97000, \"confidence\": 85, \"risk_usd\": 300, \"reasoning\": \"下跌趋势+MACD死叉\"},\n", btcEthLeverage, accountEquity*5))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\", \"reasoning\": \"止盈离场\"}\n")
	sb.WriteString("]\n```\n\n")
	sb.WriteString("**字段说明**:\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("- `confidence`: 0-100（开仓建议≥75）\n")
	sb.WriteString("- 开仓时必填: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd, reasoning\n")
	sb.WriteString("- 平仓/持有/等待时只需: symbol, action, reasoning\n\n")
	sb.WriteString("**输出要求**:\n")
	sb.WriteString("1. 先写思维链分析（可简短）\n")
	sb.WriteString("2. 然后必须输出一个有效的JSON数组，以 `[` 开始，以 `]` 结束\n")
	sb.WriteString("3. JSON数组必须在响应末尾，不能中断或截断\n")
	sb.WriteString("4. 即使所有决策都是 `wait`，也要输出JSON数组: `[{\"symbol\": \"BTCUSDT\", \"action\": \"wait\", \"reasoning\": \"无强信号\"}]`\n\n")

	// === 关键提醒 ===
	sb.WriteString("---\n\n")
	sb.WriteString("**记住**: \n")
	sb.WriteString("- 目标是夏普比率，不是交易频率\n")
	sb.WriteString("- 做空 = 做多，都是赚钱工具\n")
	sb.WriteString("- 宁可错过，不做低质量交易\n")
	sb.WriteString("- 风险回报比1:3是底线\n\n")
	
	// === 止损止盈说明 ===
	sb.WriteString("# ⚠️ 止损止盈设置（重要）\n\n")
	sb.WriteString("**做多 (open_long)**:\n")
	sb.WriteString("- 入场价: 当前市价（买在高卖更高）\n")
	sb.WriteString("- stop_loss: 入场价下方（止损价 < 入场价 < 止盈价）\n")
	sb.WriteString("- take_profit: 入场价上方\n")
	sb.WriteString("- 示例: 入场1000, 止损970, 止盈1030 → 风险30, 收益30, RR=1:1 ❌\n")
	sb.WriteString("- 正确示例: 入场1000, 止损970, 止盈1090 → 风险30, 收益90, RR=1:3 ✅\n\n")
	sb.WriteString("**做空 (open_short)**:\n")
	sb.WriteString("- 入场价: 当前市价（卖在高买更低）\n")
	sb.WriteString("- ⚠️ **CRITICAL**: stop_loss 必须大于入场价，take_profit 必须小于入场价\n")
	sb.WriteString("- stop_loss: 入场价上方（止盈价 < 入场价 < 止损价）\n")
	sb.WriteString("- take_profit: 入场价下方\n")
	sb.WriteString("- ❌ 错误示例: 入场1000, 止损970, 止盈1030 → 这是做多逻辑，做空不能用！\n")
	sb.WriteString("- ✅ 正确示例: 入场1000, 止损1030, 止盈910 → 风险30, 收益90, RR=1:3\n\n")
	sb.WriteString("**做空计算步骤（必须严格遵循）**:\n")
	sb.WriteString("1. 确定入场价（entry_price）= 当前市价\n")
	sb.WriteString("2. 计算风险点数（risk_points）= 你愿意承担的价格上涨点数\n")
	sb.WriteString("3. stop_loss = entry_price + risk_points （价格上涨触发止损）\n")
	sb.WriteString("4. take_profit = entry_price - (risk_points × 3) （价格下跌触发止盈，达到1:3风险回报比）\n")
	sb.WriteString("5. 验证: risk = stop_loss - entry_price, reward = entry_price - take_profit\n")
	sb.WriteString("6. 验证: reward / risk 必须 ≥ 3.0\n\n")
	sb.WriteString("**做空计算示例（入场价=3889.28）**:\n")
	sb.WriteString("1. entry_price = 3889.28\n")
	sb.WriteString("2. risk_points = 38.90 （假设风险）\n")
	sb.WriteString("3. stop_loss = 3889.28 + 38.90 = 3928.18 ✅（大于入场价）\n")
	sb.WriteString("4. take_profit = 3889.28 - (38.90 × 3) = 3889.28 - 116.70 = 3772.58 ✅（小于入场价）\n")
	sb.WriteString("5. risk = 3928.18 - 3889.28 = 38.90\n")
	sb.WriteString("6. reward = 3889.28 - 3772.58 = 116.70\n")
	sb.WriteString("7. RR = 116.70 / 38.90 = 3.00 ✅\n\n")
	sb.WriteString("**通用计算规则**:\n")
	sb.WriteString("- 做多: risk = entry_price - stop_loss, reward = take_profit - entry_price\n")
	sb.WriteString("- 做空: risk = stop_loss - entry_price, reward = entry_price - take_profit\n")
	sb.WriteString("- 风险回报比 = reward / risk，必须 ≥ 3.0\n")
	sb.WriteString("- ⚠️ 做空时：stop_loss > entry_price > take_profit （这是验证规则）\n")

	return sb.String()
}

// buildSystemPromptWithFallback 构建 System Prompt，优先使用模板，失败时回退到现有方法
// Uses upstream prompt_manager method as default, falls back to existing buildSystemPrompt if template is nil/not found
// templateName: 模板名称，如 "default", "adaptive", "nof1", "taro_long_prompts" (如果为空则使用 "default")
func buildSystemPromptWithFallback(accountEquity float64, btcEthLeverage, altcoinLeverage int, minPositionSizeUSD, maxPositionSizeUSD float64, templateName string) string {
	// Default to "default" if templateName is empty
	if templateName == "" {
		templateName = "default"
	}
	
	// Try to get prompt template from prompt_manager (upstream method) as default
	template, err := GetPromptTemplate(templateName)
	if err == nil && template != nil && template.Content != "" {
		// Use template from prompt_manager (upstream method) as default
		log.Printf("✓ 使用提示词模板: %s (upstream方法)", templateName)
		return template.Content
	}
	
	// Fallback to existing buildSystemPrompt behavior if template is nil/not found
	log.Printf("⚠️  提示词模板 '%s' 不可用，回退到内置prompt构建方法: %v", templateName, err)
	return buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, minPositionSizeUSD, maxPositionSizeUSD)
}

// buildUserPrompt 构建 User Prompt（动态数据）
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("**时间**: %s | **周期**: #%d | **运行**: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("**BTC**: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// 账户
	sb.WriteString(fmt.Sprintf("**账户**: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 持仓（完整市场数据）
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n")
		for i, pos := range ctx.Positions {
			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // 转换为分钟
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | 持仓时长%d分钟", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | 持仓时长%d小时%d分钟", durationHour, durationMinRemainder)
				}
			}

			sb.WriteString(fmt.Sprintf("%d. %s %s | 入场价%.4f 当前价%.4f | 盈亏%+.2f%% | 杠杆%dx | 保证金%.0f | 强平价%.4f%s\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

			// 使用FormatMarketData输出完整市场数据
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
				
				// 添加技术指标分析
				indicatorAnalysis := indicator.Analyze(marketData)
				if indicatorAnalysis != "" && indicatorAnalysis != "No significant patterns detected in recent price action." {
					sb.WriteString("\n### 📊 技术指标分析\n\n")
					sb.WriteString(indicatorAnalysis)
					sb.WriteString("\n")
				}
			}
		}
	} else {
		sb.WriteString("**当前持仓**: 无\n\n")
	}

	// 候选币种（完整市场数据）
	sb.WriteString(fmt.Sprintf("## 候选币种 (%d个)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = " (AI500+OI_Top双重信号)"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTags = " (OI_Top持仓增长)"
		}

		// 使用FormatMarketData输出完整市场数据
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
		
		// 添加技术指标分析
		indicatorAnalysis := indicator.Analyze(marketData)
		if indicatorAnalysis != "" && indicatorAnalysis != "No significant patterns detected in recent price action." {
			sb.WriteString("\n### 📊 技术指标分析\n\n")
			sb.WriteString(indicatorAnalysis)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	// 夏普比率（直接传值，不要复杂格式化）
	if ctx.Performance != nil {
		// 直接从interface{}中提取SharpeRatio
		type PerformanceData struct {
			SharpeRatio float64 `json:"sharpe_ratio"`
		}
		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				sb.WriteString(fmt.Sprintf("## 📊 夏普比率: %.2f\n\n", perfData.SharpeRatio))
			}
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("现在请分析并输出决策。\n\n")
	sb.WriteString("**必须输出格式**:\n")
	sb.WriteString("1. 思维链分析（简短即可）\n")
	sb.WriteString("2. 有效的JSON数组（以 [ 开始，以 ] 结束，包含所有决策）\n\n")
	sb.WriteString("⚠️ 记住：JSON数组是必须的，不能省略！即使没有交易决策，也要输出空的JSON数组: `[]`\n")

	return sb.String()
}

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, minPositionSizeUSD, maxPositionSizeUSD float64) (*FullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

    // 2. 提取JSON决策列表
    decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("提取决策失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

    // 3. 规范化决策：将仓位大小基于最小/最大限制进行约束（不直接拒绝，先收敛到允许范围）
    decisions = normalizeDecisions(decisions, minPositionSizeUSD, maxPositionSizeUSD)

    // 4. 验证决策
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, minPositionSizeUSD, maxPositionSizeUSD); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("决策验证失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// normalizeDecisions 将AI给出的position_size_usd在[min, max]范围内进行约束
// 注：当maxPositionSizeUSD>0时，超出部分会被自动截断至max而不是直接拒绝，以便继续后续动作
func normalizeDecisions(decisions []Decision, minPositionSizeUSD, maxPositionSizeUSD float64) []Decision {
    if len(decisions) == 0 {
        return decisions
    }

    for i := range decisions {
        // 仅对开仓动作进行规范化
        if decisions[i].Action == "open_long" || decisions[i].Action == "open_short" {
            size := decisions[i].PositionSizeUSD
            // 下限：若配置了最小仓位，且size小于下限，则提升到下限
            if minPositionSizeUSD > 0 && size > 0 && size < minPositionSizeUSD {
                decisions[i].PositionSizeUSD = minPositionSizeUSD
                // 在reasoning中追加说明（不改变AI意图，仅标注调整）
                if decisions[i].Reasoning != "" {
                    decisions[i].Reasoning += " | 已按最小仓位限制调整为 "
                }
            }
            // 上限：若配置了最大仓位，且size超过上限，则截断为上限
            if maxPositionSizeUSD > 0 && size > maxPositionSizeUSD {
                decisions[i].PositionSizeUSD = maxPositionSizeUSD
                if decisions[i].Reasoning != "" {
                    decisions[i].Reasoning += " | 已按最大仓位限制截断"
                }
            }
        }
    }
    return decisions
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 查找JSON数组的开始位置
	jsonStart := strings.Index(response, "[")

	if jsonStart > 0 {
		// 思维链是JSON数组之前的内容
		return strings.TrimSpace(response[:jsonStart])
	}

	// 如果找不到JSON，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表
func extractDecisions(response string) ([]Decision, error) {
	// 查找所有可能的JSON数组，验证哪个是决策数组
	// 决策数组应该包含对象，而不是简单的数字数组
	searchStart := 0
	for {
		arrayStart := strings.Index(response[searchStart:], "[")
		if arrayStart == -1 {
			break
		}
		arrayStart += searchStart // Adjust to absolute position

		// 从 [ 开始，匹配括号找到对应的 ]
		arrayEnd := findMatchingBracket(response, arrayStart)
		if arrayEnd == -1 {
			searchStart = arrayStart + 1
			continue
		}

		jsonContent := strings.TrimSpace(response[arrayStart : arrayEnd+1])

		// 快速检查：跳过明显不是决策数组的内容（纯数字数组）
		// 决策数组应该包含 "symbol" 或 "action" 等关键字
		if !strings.Contains(jsonContent, "\"symbol\"") && !strings.Contains(jsonContent, "\"action\"") {
			// 这可能是价格数据数组，跳过
			searchStart = arrayEnd + 1
			continue
		}

		// 🔧 修复常见的JSON格式错误：缺少引号的字段值
		// 匹配: "reasoning": 内容"}  或  "reasoning": 内容}  (没有引号)
		// 修复为: "reasoning": "内容"}
		// 使用简单的字符串扫描而不是正则表达式
		jsonContent = fixMissingQuotes(jsonContent)

		// 🔧 修复算术表达式：将 JSON 中的计算表达式（如 "150 * (0.62 - 0.61) * 5"）替换为计算结果
		// 例如: "risk_usd": 150 * (0.62 - 0.61) * 5  ->  "risk_usd": 0.75
		jsonContent = fixArithmeticExpressions(jsonContent)

		// 解析JSON
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err == nil {
			// 验证这是一个有效的决策数组：至少有一个决策，且有symbol字段
			if len(decisions) > 0 && decisions[0].Symbol != "" {
				return decisions, nil
			}
		}

		// 如果解析失败或验证失败，继续查找下一个数组
		searchStart = arrayEnd + 1
	}

	// 如果所有数组都解析失败，尝试最后一个找到的数组（向后兼容）
	arrayStart := strings.LastIndex(response, "[")
	if arrayStart == -1 {
		// Fallback: 如果没有找到JSON数组，返回一个wait决策而不是报错
		// 这样可以避免系统崩溃，让AI在下个周期重试
		log.Printf("⚠️ 警告: AI响应中未找到JSON数组，返回wait决策")
		return []Decision{
			{
				Symbol:   "",
				Action:   "wait",
				Reasoning: "AI响应格式错误，未找到JSON数组",
			},
		}, nil
	}

	arrayEnd := findMatchingBracket(response, arrayStart)
	if arrayEnd == -1 {
		// Fallback: 如果找到了[但没有找到]，也返回wait决策
		log.Printf("⚠️ 警告: AI响应中JSON数组不完整（找到[但未找到]），返回wait决策")
		return []Decision{
			{
				Symbol:   "",
				Action:   "wait",
				Reasoning: "AI响应格式错误，JSON数组不完整",
			},
		}, nil
	}

	jsonContent := strings.TrimSpace(response[arrayStart : arrayEnd+1])
	jsonContent = fixMissingQuotes(jsonContent)
	jsonContent = fixArithmeticExpressions(jsonContent)

	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		// 即使JSON解析失败，也返回wait决策而不是报错
		log.Printf("⚠️ 警告: JSON解析失败: %v，返回wait决策\nJSON内容: %s", err, jsonContent)
		return []Decision{
			{
				Symbol:   "",
				Action:   "wait",
				Reasoning: fmt.Sprintf("JSON解析失败: %v", err),
			},
		}, nil
	}

	return decisions, nil
}

// fixMissingQuotes 替换中文引号为英文引号（避免输入法自动转换）
func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '
	return jsonStr
}

// fixArithmeticExpressions 修复JSON中的算术表达式
// 例如: "risk_usd": 150 * (0.62 - 0.61) * 5  ->  "risk_usd": 150
// 匹配数值字段后的算术表达式，并移除它们，只保留第一个数字
func fixArithmeticExpressions(jsonStr string) string {
	// 匹配模式: "field_name": number * expression 或 "field_name": number ( expression )
	// 例如: "risk_usd": 150 * (0.62 - 0.61) * 5
	// 匹配: "字段名": 数字后面跟着运算符和表达式（直到逗号、}、]或换行）
	
	// 正则表达式：匹配 "字段名": 数字，后面跟着运算符和表达式
	// 模式: "字段名": 数字 (空格 运算符 表达式) 
	// 注意：表达式可能包含括号、数字、运算符、空格
	// 使用非贪婪匹配直到遇到逗号、右括号或换行
	arithmeticPattern := regexp.MustCompile(`("(?:risk_usd|position_size_usd|stop_loss|take_profit|leverage|confidence)"\s*:\s*)([\d.]+)\s*([*+\-/\s()\d.]+?)(\s*[,}\]\n])`)
	
	jsonStr = arithmeticPattern.ReplaceAllStringFunc(jsonStr, func(match string) string {
		// 提取字段名、第一个数字、表达式部分和结尾字符
		submatches := arithmeticPattern.FindStringSubmatch(match)
		if len(submatches) < 5 {
			return match // 无法解析，返回原字符串
		}
		
		fieldPart := submatches[1]     // "risk_usd": 
		firstNum := submatches[2]      // 第一个数字，如 "150"
		expression := submatches[3]    // 后面的表达式，如 " * (0.62 - 0.61) * 5"
		endingChar := submatches[4]    // 结尾字符：逗号、}、]或换行
		
		// 如果表达式包含算术运算符（*、/、+、-、()），说明这是一个计算表达式
		// 为了安全，我们只保留第一个数字，移除后面的计算表达式
		// 因为 risk_usd 是可选字段，且AI应该在思维链中说明计算逻辑，JSON中只应该包含最终数值
		if strings.ContainsAny(expression, "*+-/()") {
			// 移除表达式，只保留字段名、第一个数字和结尾字符
			return fieldPart + firstNum + endingChar
		}
		
		return match // 没有运算符，返回原字符串
	})
	
	return jsonStr
}

// validateDecisions 验证所有决策（需要账户信息和杠杆配置）
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, minPositionSizeUSD, maxPositionSizeUSD float64) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage, minPositionSizeUSD, maxPositionSizeUSD); err != nil {
			return fmt.Errorf("决策 #%d 验证失败: %w", i+1, err)
		}
	}
	return nil
}

// findMatchingBracket 查找匹配的右括号
func findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// validateDecision 验证单个决策的有效性
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, minPositionSizeUSD, maxPositionSizeUSD float64) error {
	// 验证action
	validActions := map[string]bool{
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: %s", d.Action)
	}

	// 开仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" {
		// 根据币种使用配置的杠杆上限
		maxLeverage := altcoinLeverage          // 山寨币使用配置的杠杆
		maxPositionValue := accountEquity * 1.5 // 山寨币最多1.5倍账户净值
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC和ETH使用配置的杠杆
			maxPositionValue = accountEquity * 10 // BTC/ETH最多10倍账户净值
		}

		if d.Leverage <= 0 || d.Leverage > maxLeverage {
			return fmt.Errorf("杠杆必须在1-%d之间（%s，当前配置上限%d倍）: %d", maxLeverage, d.Symbol, maxLeverage, d.Leverage)
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("仓位大小必须大于0: %.2f", d.PositionSizeUSD)
		}

		// 验证最小仓位大小（USD）
		if minPositionSizeUSD > 0 && d.PositionSizeUSD < minPositionSizeUSD {
			return fmt.Errorf("仓位大小 %.2f USDT 低于最小限制 %.2f USDT", d.PositionSizeUSD, minPositionSizeUSD)
		}

		// 验证最大仓位大小（USD）- 优先使用配置的USD限制，否则使用账户净值倍数限制
		if maxPositionSizeUSD > 0 {
			if d.PositionSizeUSD > maxPositionSizeUSD {
				return fmt.Errorf("仓位大小 %.2f USDT 超过最大限制 %.2f USDT", d.PositionSizeUSD, maxPositionSizeUSD)
			}
		} else {
			// 如果没有配置USD限制，使用账户净值倍数限制（加1%容差以避免浮点数精度问题）
			tolerance := maxPositionValue * 0.01 // 1%容差
			if d.PositionSizeUSD > maxPositionValue+tolerance {
				if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
					return fmt.Errorf("BTC/ETH单币种仓位价值不能超过%.0f USDT（10倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
				} else {
					return fmt.Errorf("山寨币单币种仓位价值不能超过%.0f USDT（1.5倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
				}
			}
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("止损和止盈必须大于0")
		}

		// 验证止损止盈的合理性
		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("做多时止损价必须小于止盈价（当前止损%.2f >= 止盈%.2f）。做多逻辑：stop_loss < entry < take_profit", d.StopLoss, d.TakeProfit)
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("做空时止损价必须大于止盈价（当前止损%.2f <= 止盈%.2f）。做空计算：stop_loss = entry + risk_points, take_profit = entry - (risk_points × 3)。正确逻辑：take_profit < entry < stop_loss", d.StopLoss, d.TakeProfit)
			}
		}

		// 验证风险回报比（必须≥1:3）
		// 计算入场价（假设当前市价）
		var entryPrice float64
		if d.Action == "open_long" {
			// 做多：入场价在止损和止盈之间
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2 // 假设在20%位置入场
		} else {
			// 做空：入场价在止损和止盈之间
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2 // 假设在20%位置入场
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		// 硬约束：风险回报比必须≥3.0
		if riskRewardRatio < 3.0 {
			return fmt.Errorf("风险回报比过低(%.2f:1)，必须≥3.0:1 [风险:%.2f%% 收益:%.2f%%] [止损:%.2f 止盈:%.2f]",
				riskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	return nil
}
