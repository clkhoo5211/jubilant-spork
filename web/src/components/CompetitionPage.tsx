import { useState } from "react";
import useSWR from "swr";
import { api } from "../lib/api";
import type { CompetitionData } from "../types";
import { ComparisonChart } from "./ComparisonChart";
import { CompetitionModelChat } from "./CompetitionModelChat";
import { CompetitionPositions } from "./CompetitionPositions";
import { useLanguage } from "../contexts/LanguageContext";
import { t } from "../i18n/translations";
import { getTraderColor } from "../utils/traderColors";

// Get model icon helper
const getModelIcon = (model: string) => {
  const m = model.toUpperCase();
  if (m.includes('DEEPSEEK')) return '🐋';
  if (m.includes('QWEN')) return '💜';
  if (m.includes('CLAUDE')) return '🧡';
  if (m.includes('GPT')) return '🟢';
  if (m.includes('GEMINI')) return '🔵';
  if (m.includes('GROK')) return '⚡';
  return '🤖';
};

// Get coin icon - returns either SVG path or emoji (same as CompetitionPositions)
const getCoinIcon = (symbol: string): { type: 'svg' | 'emoji'; value: string } => {
  const baseSymbol = symbol.replace('USDT', '').toUpperCase();
  
  // All coins we have SVG files for (from nof1.ai and CoinGecko)
  const availableCoins = [
    'BTC', 'ETH', 'SOL', 'BNB', 'XRP', 'DOGE',  // From nof1.ai
    'ADA', 'DOT', 'MATIC', 'LINK', 'UNI', 'AVAX',  // From GitHub crypto-icons
    'LTC', 'BCH', 'XLM', 'ATOM', 'ICP', 'FIL',  // From GitHub crypto-icons
    'ARB', 'OP', 'SUI', 'APT', 'INJ', 'SEI', 'TIA'  // From CoinGecko/alternative sources
  ];
  const hasLocalIcon = availableCoins.includes(baseSymbol);
  
  if (hasLocalIcon) {
    return { type: 'svg', value: `/coins/${baseSymbol.toLowerCase()}.svg` };
  }
  
  // Fallback to emoji for coins without local SVG
  const iconMap: { [key: string]: string } = {
    // Keep emoji fallbacks for any coins not in our collection
  };
  
  return { type: 'emoji', value: iconMap[baseSymbol] || '◈' };
};

export function CompetitionPage() {
  const { language } = useLanguage();
  const [activeTab, setActiveTab] = useState<
    "leaderboard" | "positions" | "chat"
  >("leaderboard");
  const [leaderboardSubTab, setLeaderboardSubTab] = useState<
    "overall" | "advanced"
  >("overall");
  const [selectedTraderId, setSelectedTraderId] = useState<string | null>(null);
  const { data: competition } = useSWR<CompetitionData>(
    "competition",
    api.getCompetition,
    {
      refreshInterval: 15000, // 15秒刷新（竞赛数据不需要太频繁更新）
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    },
  );

  // Calculate leader early for use in hooks
  const leader = competition?.traders && competition.traders.length > 0
    ? [...competition.traders].sort((a, b) => b.total_pnl_pct - a.total_pnl_pct)[0]
    : null;

  // Fetch performance data for all traders (for Advanced Analytics)
  const { data: allPerformance } = useSWR(
    competition?.traders && competition.traders.length > 0
      ? `competition-performance-${competition.traders.map(t => t.trader_id).sort().join(',')}`
      : null,
    async () => {
      const promises = competition!.traders.map(async (trader) => {
        try {
          const perf = await api.getPerformance(trader.trader_id);
          return { traderId: trader.trader_id, performance: perf };
        } catch (err) {
          console.error(`Failed to fetch performance for ${trader.trader_id}:`, err);
          return { traderId: trader.trader_id, performance: null };
        }
      });
      return Promise.all(promises);
    },
    {
      refreshInterval: 30000,
      revalidateOnFocus: false,
    }
  );

  // Fetch leader's positions for coin icons display
  const { data: leaderPositions } = useSWR(
    leader ? `leader-positions-${leader.trader_id}` : null,
    async () => {
      try {
        const positions = await api.getPositions(leader!.trader_id);
        // Handle null/undefined or non-array responses
        if (!positions || !Array.isArray(positions)) {
          return [];
        }
        // Get unique coin symbols
        const uniqueSymbols = Array.from(new Set(positions.map(p => p.symbol)));
        return uniqueSymbols.slice(0, 6); // Show max 6 coins
      } catch (err) {
        console.error(`Failed to fetch leader positions:`, err);
        return [];
      }
    },
    {
      refreshInterval: 15000,
      revalidateOnFocus: false,
    }
  );

  if (!competition || !competition.traders) {
    return (
      <div className="space-y-6">
        <div className="binance-card p-8 animate-pulse">
          <div className="flex items-center justify-between mb-6">
            <div className="space-y-3 flex-1">
              <div className="skeleton h-8 w-64"></div>
              <div className="skeleton h-4 w-48"></div>
            </div>
            <div className="skeleton h-12 w-32"></div>
          </div>
        </div>
        <div className="binance-card p-6">
          <div className="skeleton h-6 w-40 mb-4"></div>
          <div className="space-y-3">
            <div className="skeleton h-20 w-full rounded"></div>
            <div className="skeleton h-20 w-full rounded"></div>
          </div>
        </div>
      </div>
    );
  }

  // 按收益率排序
  const sortedTraders = [...competition.traders].sort(
    (a, b) => b.total_pnl_pct - a.total_pnl_pct,
  );

  // 动态生成对战标签：获取所有trader的model_name
  const getBattleLabel = () => {
    const models = competition.traders.map((t) => t.ai_model.toUpperCase());
    if (models.length === 2) {
      return `${models[0]} vs ${models[1]}`;
    } else if (models.length > 2) {
      return `${models.slice(0, -1).join(", ")} vs ${models[models.length - 1]}`;
    } else if (models.length === 1) {
      return models[0];
    }
    return "AI Competition";
  };

  return (
    <div className="space-y-5 animate-fade-in">
      {/* Competition Header - 精简版 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div
            className="w-12 h-12 rounded-xl flex items-center justify-center text-2xl"
            style={{
              background: "linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)",
              boxShadow: "0 4px 14px rgba(240, 185, 11, 0.4)",
            }}
          >
            🏆
          </div>
          <div>
            <h1
              className="text-2xl font-bold flex items-center gap-2"
              style={{ color: "#EAECEF" }}
            >
              {t("aiCompetition", language)}
              <span
                className="text-xs font-normal px-2 py-1 rounded"
                style={{
                  background: "rgba(240, 185, 11, 0.15)",
                  color: "#F0B90B",
                }}
              >
                {competition.count} {t("traders", language)}
              </span>
            </h1>
            <p className="text-xs" style={{ color: "#848E9C" }}>
              {getBattleLabel()} · {language === "en" ? "Real-time" : "实时"}
            </p>
          </div>
        </div>
        <div className="text-right">
          <div className="text-xs mb-1" style={{ color: "#848E9C" }}>
            {t("leader", language)}
          </div>
          <div className="text-lg font-bold" style={{ color: "#F0B90B" }}>
            {leader?.trader_name}
          </div>
          <div
            className="text-sm font-semibold"
            style={{
              color: (leader?.total_pnl ?? 0) >= 0 ? "#0ECB81" : "#F6465D",
            }}
          >
            {(leader?.total_pnl ?? 0) >= 0 ? "+" : ""}
            {leader?.total_pnl_pct?.toFixed(2) || "0.00"}%
          </div>
        </div>
      </div>

      {/* Side-by-Side Layout: Performance Chart + Tabbed Content */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Left: Performance Comparison Chart */}
        <div
          className="binance-card p-5 animate-slide-in"
          style={{ animationDelay: "0.1s" }}
        >
          <div className="flex items-center justify-between mb-4">
            <h2
              className="text-lg font-bold flex items-center gap-2"
              style={{ color: "#EAECEF" }}
            >
              {t("performanceComparison", language)}
            </h2>
            <div className="flex items-center gap-2">
              {selectedTraderId && (
                <button
                  onClick={() => setSelectedTraderId(null)}
                  className="text-xs px-3 py-1 rounded font-semibold transition-all hover:scale-105"
                  style={{
                    background: "rgba(246, 70, 93, 0.15)",
                    color: "#F6465D",
                    border: "1px solid rgba(246, 70, 93, 0.3)",
                  }}
                >
                  {language === "en" ? "Show All" : "显示全部"}
                </button>
              )}
              <div
                className="text-xs px-2 py-1 rounded"
                style={{
                  background: "rgba(240, 185, 11, 0.1)",
                  color: "#F0B90B",
                  border: "1px solid rgba(240, 185, 11, 0.2)",
                }}
              >
                {t("live", language)}
              </div>
            </div>
          </div>
          <ComparisonChart 
            traders={selectedTraderId 
              ? sortedTraders.filter(t => t.trader_id === selectedTraderId)
              : sortedTraders
            } 
          />
        </div>

        {/* Right: Tabbed View - Leaderboard, Positions, Chat */}
        <div
          className="binance-card animate-slide-in"
          style={{ animationDelay: "0.1s" }}
        >
          {/* Tab Navigation */}
          <div className="border-b" style={{ borderColor: "#2B3139" }}>
            <div className="flex gap-2">
              <button
                onClick={() => setActiveTab("leaderboard")}
                className="px-6 py-4 font-semibold transition-all duration-200 relative"
                style={{
                  color: activeTab === "leaderboard" ? "#F0B90B" : "#848E9C",
                  borderBottom:
                    activeTab === "leaderboard"
                      ? "2px solid #F0B90B"
                      : "2px solid transparent",
                }}
                onMouseEnter={(e) => {
                  if (activeTab !== "leaderboard") {
                    e.currentTarget.style.color = "#EAECEF";
                  }
                }}
                onMouseLeave={(e) => {
                  if (activeTab !== "leaderboard") {
                    e.currentTarget.style.color = "#848E9C";
                  }
                }}
              >
                🏆 {t("leaderboardTab", language)}
              </button>
              <button
                onClick={() => setActiveTab("positions")}
                className="px-6 py-4 font-semibold transition-all duration-200"
                style={{
                  color: activeTab === "positions" ? "#F0B90B" : "#848E9C",
                  borderBottom:
                    activeTab === "positions"
                      ? "2px solid #F0B90B"
                      : "2px solid transparent",
                }}
                onMouseEnter={(e) => {
                  if (activeTab !== "positions") {
                    e.currentTarget.style.color = "#EAECEF";
                  }
                }}
                onMouseLeave={(e) => {
                  if (activeTab !== "positions") {
                    e.currentTarget.style.color = "#848E9C";
                  }
                }}
              >
                📊 {t("positionsTab", language)}
              </button>
              <button
                onClick={() => setActiveTab("chat")}
                className="px-6 py-4 font-semibold transition-all duration-200"
                style={{
                  color: activeTab === "chat" ? "#F0B90B" : "#848E9C",
                  borderBottom:
                    activeTab === "chat"
                      ? "2px solid #F0B90B"
                      : "2px solid transparent",
                }}
                onMouseEnter={(e) => {
                  if (activeTab !== "chat") {
                    e.currentTarget.style.color = "#EAECEF";
                  }
                }}
                onMouseLeave={(e) => {
                  if (activeTab !== "chat") {
                    e.currentTarget.style.color = "#848E9C";
                  }
                }}
              >
                💬 {t("chatTab", language)}
              </button>
            </div>
          </div>

          {/* Tab Content */}
          <div className="p-6">
            {activeTab === "leaderboard" && (
              <div className="space-y-3">
                {/* Leaderboard Sub-Tabs */}
                <div className="flex gap-1 border-b" style={{ borderColor: "#2B3139" }}>
                  <button
                    onClick={() => setLeaderboardSubTab("overall")}
                    className="px-4 py-2 text-sm font-semibold transition-all duration-200"
                    style={{
                      color: leaderboardSubTab === "overall" ? "#EAECEF" : "#848E9C",
                      borderBottom: leaderboardSubTab === "overall" ? "2px solid #F0B90B" : "2px solid transparent",
                    }}
                  >
                    {language === "en" ? "OVERALL STATS" : "总体统计"}
                  </button>
                  <button
                    onClick={() => setLeaderboardSubTab("advanced")}
                    className="px-4 py-2 text-sm font-semibold transition-all duration-200"
                    style={{
                      color: leaderboardSubTab === "advanced" ? "#EAECEF" : "#848E9C",
                      borderBottom: leaderboardSubTab === "advanced" ? "2px solid #F0B90B" : "2px solid transparent",
                    }}
                  >
                    {language === "en" ? "ADVANCED ANALYTICS" : "高级分析"}
                  </button>
                </div>

                {/* Leaderboard Content */}
                {leaderboardSubTab === "overall" && (
                  <div className="space-y-2">
                    {sortedTraders.map((trader, index) => {
                      const isLeader = index === 0;
                      const isSelected = selectedTraderId === trader.trader_id;
                      const traderColor = getTraderColor(
                        sortedTraders,
                        trader.trader_id,
                      );

                      return (
                        <button
                          key={trader.trader_id}
                          onClick={() => setSelectedTraderId(isSelected ? null : trader.trader_id)}
                          className="w-full text-left rounded p-3 transition-all duration-300 hover:translate-y-[-1px] cursor-pointer"
                          style={{
                            background: isSelected
                              ? `linear-gradient(135deg, ${traderColor}20 0%, #0B0E11 100%)`
                              : isLeader
                              ? "linear-gradient(135deg, rgba(240, 185, 11, 0.08) 0%, #0B0E11 100%)"
                              : "#0B0E11",
                            border: `1px solid ${
                              isSelected
                                ? traderColor
                                : isLeader
                                ? "rgba(240, 185, 11, 0.4)"
                                : "#2B3139"
                            }`,
                            boxShadow: isSelected
                              ? `0 3px 15px ${traderColor}30, 0 0 0 1px ${traderColor}50`
                              : isLeader
                              ? "0 3px 15px rgba(240, 185, 11, 0.12), 0 0 0 1px rgba(240, 185, 11, 0.15)"
                              : "0 1px 4px rgba(0, 0, 0, 0.3)",
                          }}
                        >
                          <div className="flex items-center justify-between">
                            {/* Rank & Name */}
                            <div className="flex items-center gap-3">
                              <div className="text-2xl w-6">
                                {index === 0 ? "🥇" : index === 1 ? "🥈" : "🥉"}
                              </div>
                              <div>
                                <div
                                  className="font-bold text-sm"
                                  style={{ color: "#EAECEF" }}
                                >
                                  {trader.trader_name}
                                </div>
                                <div
                                  className="text-xs mono font-semibold"
                                  style={{ color: traderColor }}
                                >
                                  {trader.ai_model.toUpperCase()}
                                </div>
                              </div>
                            </div>

                            {/* Stats */}
                            <div className="flex items-center gap-3">
                              {/* Total Equity */}
                              <div className="text-right">
                                <div
                                  className="text-xs"
                                  style={{ color: "#848E9C" }}
                                >
                                  {t("equity", language)}
                                </div>
                                <div
                                  className="text-sm font-bold mono"
                                  style={{ color: "#EAECEF" }}
                                >
                                  {trader.total_equity?.toFixed(2) || "0.00"}
                                </div>
                              </div>

                              {/* P&L */}
                              <div className="text-right min-w-[90px]">
                                <div
                                  className="text-xs"
                                  style={{ color: "#848E9C" }}
                                >
                                  {t("pnl", language)}
                                </div>
                                <div
                                  className="text-lg font-bold mono"
                                  style={{
                                    color:
                                      (trader.total_pnl ?? 0) >= 0
                                        ? "#0ECB81"
                                        : "#F6465D",
                                  }}
                                >
                                  {(trader.total_pnl ?? 0) >= 0 ? "+" : ""}
                                  {trader.total_pnl_pct?.toFixed(2) || "0.00"}%
                                </div>
                                <div
                                  className="text-xs mono"
                                  style={{ color: "#848E9C" }}
                                >
                                  {(trader.total_pnl ?? 0) >= 0 ? "+" : ""}
                                  {trader.total_pnl?.toFixed(2) || "0.00"}
                                </div>
                              </div>

                              {/* Positions */}
                              <div className="text-right">
                                <div
                                  className="text-xs"
                                  style={{ color: "#848E9C" }}
                                >
                                  {t("pos", language)}
                                </div>
                                <div
                                  className="text-sm font-bold mono"
                                  style={{ color: "#EAECEF" }}
                                >
                                  {trader.position_count}
                                </div>
                                <div
                                  className="text-xs"
                                  style={{ color: "#848E9C" }}
                                >
                                  {trader.margin_used_pct.toFixed(1)}%
                                </div>
                              </div>

                              {/* Status */}
                              <div>
                                <div
                                  className="px-2 py-1 rounded text-xs font-bold"
                                  style={
                                    trader.is_running
                                      ? {
                                          background: "rgba(14, 203, 129, 0.1)",
                                          color: "#0ECB81",
                                        }
                                      : {
                                          background: "rgba(246, 70, 93, 0.1)",
                                          color: "#F6465D",
                                        }
                                  }
                                >
                                  {trader.is_running ? "●" : "○"}
                                </div>
                              </div>
                            </div>
                          </div>
                        </button>
                      );
                    })}
                  </div>
                )}

                {leaderboardSubTab === "advanced" && (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b" style={{ borderColor: "#2B3139" }}>
                          <th className="pb-3 px-2 text-left font-semibold text-xs" style={{ color: "#848E9C" }}>
                            RANK
                          </th>
                          <th className="pb-3 px-2 text-left font-semibold text-xs" style={{ color: "#848E9C" }}>
                            MODEL
                          </th>
                          <th className="pb-3 px-2 text-left font-semibold text-xs" style={{ color: "#848E9C" }}>
                            ACCT VALUE
                          </th>
                          <th className="pb-3 px-2 text-left font-semibold text-xs" style={{ color: "#848E9C" }}>
                            WIN RATE
                          </th>
                          <th className="pb-3 px-2 text-left font-semibold text-xs" style={{ color: "#848E9C" }}>
                            AVG WIN
                          </th>
                          <th className="pb-3 px-2 text-left font-semibold text-xs" style={{ color: "#848E9C" }}>
                            AVG LOSS
                          </th>
                          <th className="pb-3 px-2 text-left font-semibold text-xs" style={{ color: "#848E9C" }}>
                            PROFIT FACTOR
                          </th>
                          <th className="pb-3 px-2 text-left font-semibold text-xs" style={{ color: "#848E9C" }}>
                            SHARPE
                          </th>
                          <th className="pb-3 px-2 text-left font-semibold text-xs" style={{ color: "#848E9C" }}>
                            TRADES
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {sortedTraders.map((trader, index) => {
                          const perf = allPerformance?.find(p => p.traderId === trader.trader_id);
                          const traderColor = getTraderColor(sortedTraders, trader.trader_id);
                          const isSelected = selectedTraderId === trader.trader_id;
                          
                          return (
                            <tr
                              key={trader.trader_id}
                              onClick={() => setSelectedTraderId(isSelected ? null : trader.trader_id)}
                              className="border-b last:border-0 cursor-pointer transition-all hover:bg-opacity-50"
                              style={{ 
                                borderColor: isSelected ? traderColor : "#2B3139",
                                background: isSelected ? `${traderColor}10` : "transparent"
                              }}
                            >
                              <td className="py-3 px-2">
                                <span className="text-base">
                                  {index === 0 ? "🥇" : index === 1 ? "🥈" : index === 2 ? "🥉" : `${index + 1}.`}
                                </span>
                              </td>
                              <td className="py-3 px-2">
                                <div className="flex items-center gap-2">
                                  <div
                                    className="w-8 h-8 rounded flex items-center justify-center text-sm font-bold flex-shrink-0"
                                    style={{
                                      background: `linear-gradient(135deg, ${traderColor} 0%, ${traderColor}dd 100%)`,
                                      border: `1px solid ${traderColor}`,
                                    }}
                                  >
                                    {getModelIcon(trader.ai_model)}
                                  </div>
                                  <div className="font-bold text-xs" style={{ color: "#EAECEF" }}>
                                    {trader.ai_model.toUpperCase().replace(/-/g, ' ')}
                                  </div>
                                </div>
                              </td>
                              <td className="py-3 px-2 font-mono font-bold text-xs" style={{ color: "#EAECEF" }}>
                                ${trader.total_equity?.toFixed(2) || "0.00"}
                              </td>
                              <td className="py-3 px-2">
                                <span
                                  className="font-semibold text-xs"
                                  style={{ color: (perf?.performance?.win_rate || 0) >= 50 ? "#0ECB81" : "#848E9C" }}
                                >
                                  {perf?.performance?.win_rate ? `${perf.performance.win_rate.toFixed(1)}%` : "N/A"}
                                </span>
                              </td>
                              <td className="py-3 px-2 font-mono text-xs" style={{ color: "#0ECB81" }}>
                                {perf?.performance?.avg_win ? `$${perf.performance.avg_win.toFixed(2)}` : "N/A"}
                              </td>
                              <td className="py-3 px-2 font-mono text-xs" style={{ color: "#F6465D" }}>
                                {perf?.performance?.avg_loss ? `-$${Math.abs(perf.performance.avg_loss).toFixed(2)}` : "N/A"}
                              </td>
                              <td className="py-3 px-2">
                                <span
                                  className="font-semibold text-xs"
                                  style={{ color: (perf?.performance?.profit_factor || 0) >= 1.5 ? "#0ECB81" : "#848E9C" }}
                                >
                                  {perf?.performance?.profit_factor ? perf.performance.profit_factor.toFixed(2) : "N/A"}
                                </span>
                              </td>
                              <td className="py-3 px-2">
                                <span
                                  className="font-semibold text-xs"
                                  style={{ color: (perf?.performance?.sharpe_ratio || 0) >= 0.5 ? "#0ECB81" : "#848E9C" }}
                                >
                                  {perf?.performance?.sharpe_ratio ? perf.performance.sharpe_ratio.toFixed(3) : "N/A"}
                                </span>
                              </td>
                              <td className="py-3 px-2 font-mono text-xs" style={{ color: "#848E9C" }}>
                                {perf?.performance?.total_trades || "0"}
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            )}

            {activeTab === "positions" && (
              <CompetitionPositions
                traderIds={competition.traders.map((t) => t.trader_id)}
                traders={competition.traders}
              />
            )}

            {activeTab === "chat" && (
              <CompetitionModelChat
                traderIds={competition.traders.map((t) => t.trader_id)}
                maxMessages={50}
                autoRefresh={true}
              />
            )}
          </div>
        </div>
      </div>

      {/* Winning Model Summary + Leaderboard Bar Chart */}
      {leader && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-5 animate-slide-in" style={{ animationDelay: '0.3s' }}>
          {/* Left: Winning Model Summary */}
          <div className="binance-card p-5">
            <div className="text-xs uppercase font-bold mb-3" style={{ color: "#848E9C" }}>
              {language === "en" ? "WINNING MODEL" : "领先模型"}
            </div>
            <div className="flex items-center gap-3 mb-4">
              <div
                className="w-10 h-10 rounded flex items-center justify-center text-xl flex-shrink-0"
                style={{
                  background: `linear-gradient(135deg, ${getTraderColor(sortedTraders, leader.trader_id)} 0%, ${getTraderColor(sortedTraders, leader.trader_id)}dd 100%)`,
                  border: `2px solid ${getTraderColor(sortedTraders, leader.trader_id)}`,
                }}
              >
                {getModelIcon(leader.ai_model)}
              </div>
              <div>
                <div className="text-lg font-bold" style={{ color: "#EAECEF" }}>
                  {leader.ai_model.toUpperCase().replace(/-/g, ' ')}
                </div>
                <div className="text-sm" style={{ color: "#848E9C" }}>
                  {leader.trader_name}
                </div>
              </div>
            </div>
            <div className="mb-4">
              <div className="text-xs uppercase font-bold mb-2" style={{ color: "#848E9C" }}>
                {t("totalEquity", language)}
              </div>
              <div className="text-2xl font-bold font-mono" style={{ color: "#EAECEF" }}>
                ${leader.total_equity?.toFixed(2) || "0.00"}
              </div>
            </div>
            {/* Active Positions Summary */}
            {leader.position_count > 0 && (
              <div>
                <div className="text-xs uppercase font-bold mb-2" style={{ color: "#848E9C" }}>
                  {t("activePositions", language)}
                </div>
                <div className="flex items-center gap-2 flex-wrap">
                  <div className="flex items-center gap-1">
                    <div className="text-2xl">●</div>
                    <div className="text-sm font-bold" style={{ color: "#EAECEF" }}>
                      {leader.position_count}
                    </div>
                  </div>
                  {leaderPositions && leaderPositions.length > 0 && (
                    <div className="flex items-center gap-1.5 flex-wrap ml-2">
                      {leaderPositions.map((symbol) => {
                        const icon = getCoinIcon(symbol);
                        return (
                          <div
                            key={symbol}
                            className="flex items-center gap-1 px-2 py-1 rounded"
                            style={{
                              background: 'rgba(240, 185, 11, 0.1)',
                              border: '1px solid rgba(240, 185, 11, 0.2)',
                            }}
                            title={symbol}
                          >
                            {icon.type === 'svg' ? (
                              <div 
                                className="flex-shrink-0 relative"
                                style={
                                  symbol.toUpperCase().includes('XRP')
                                    ? {
                                        background: 'rgba(255, 255, 255, 0.15)',
                                        borderRadius: '50%',
                                        padding: '2px',
                                        width: '20px',
                                        height: '20px',
                                        display: 'flex',
                                        alignItems: 'center',
                                        justifyContent: 'center',
                                      }
                                    : {}
                                }
                              >
                                <img 
                                  src={icon.value} 
                                  alt={symbol}
                                  className="w-4 h-4 flex-shrink-0"
                                  style={
                                    symbol.toUpperCase().includes('XRP')
                                      ? {
                                          filter: 'brightness(1.2) contrast(1.1)',
                                        }
                                      : {}
                                  }
                                  onError={(e) => {
                                    const target = e.target as HTMLImageElement;
                                    target.style.display = 'none';
                                    const fallback = document.createElement('span');
                                    fallback.className = 'text-xs';
                                    fallback.textContent = '◈';
                                    target.parentNode?.insertBefore(fallback, target);
                                  }}
                                />
                              </div>
                            ) : (
                              <span className="text-xs">{icon.value}</span>
                            )}
                            <span className="text-xs font-mono font-semibold" style={{ color: '#F0B90B' }}>
                              {symbol.replace('USDT', '')}
                            </span>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>

          {/* Right: Leaderboard Bar Chart */}
          <div className="lg:col-span-2 binance-card p-5">
            <div className="space-y-3">
              {sortedTraders.map((trader) => {
                const traderColor = getTraderColor(sortedTraders, trader.trader_id);
                const maxEquity = Math.max(...sortedTraders.map(t => t.total_equity || 0));
                const barWidth = maxEquity > 0 ? (trader.total_equity || 0) / maxEquity * 100 : 0;
                
                return (
                  <div key={trader.trader_id} className="relative">
                    <div className="flex items-center justify-between mb-1">
                      <div className="flex items-center gap-2">
                        <div
                          className="w-8 h-8 rounded flex items-center justify-center text-sm flex-shrink-0"
                          style={{
                            background: `linear-gradient(135deg, ${traderColor} 0%, ${traderColor}dd 100%)`,
                            border: `1px solid ${traderColor}`,
                          }}
                        >
                          {getModelIcon(trader.ai_model)}
                        </div>
                        <div className="text-sm font-bold" style={{ color: "#EAECEF" }}>
                          {trader.ai_model.toUpperCase().replace(/-/g, ' ').substring(0, 15)}
                          {trader.ai_model.length > 15 ? '...' : ''}
                        </div>
                      </div>
                      <div className="text-sm font-bold font-mono" style={{ color: "#EAECEF" }}>
                        ${trader.total_equity?.toFixed(2) || "0.00"}
                      </div>
                    </div>
                    <div className="relative h-6 rounded overflow-hidden" style={{ background: "#1E2329" }}>
                      <div
                        className="absolute top-0 left-0 h-full transition-all duration-500"
                        style={{
                          width: `${barWidth}%`,
                          background: `linear-gradient(90deg, ${traderColor} 0%, ${traderColor}cc 100%)`,
                        }}
                      />
                      <div
                        className="absolute top-0 right-0 h-full"
                        style={{
                          width: `${100 - barWidth}%`,
                          background: "#2B3139",
                        }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
