import { useState } from 'react';
import useSWR from 'swr';
import { api } from '../lib/api';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';
import { getTraderColor } from '../utils/traderColors';
import type { Position, CompetitionTraderData } from '../types';

// Get coin icon - returns either SVG path or emoji
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

interface CompetitionPositionsProps {
  traderIds: string[];
  traders: CompetitionTraderData[];
}

interface TraderPositions {
  trader_id: string;
  trader_name: string;
  ai_model: string;
  positions: Position[];
  totalUnrealizedPnL: number;
  availableBalance: number;
}

interface ExitPlanModal {
  isOpen: boolean;
  position: Position | null;
  traderName: string;
  stopLoss?: number;
  takeProfit?: number;
}

export function CompetitionPositions({ traderIds, traders }: CompetitionPositionsProps) {
  const { language } = useLanguage();
  const [selectedModel, setSelectedModel] = useState<string>('ALL');
  const [modal, setModal] = useState<ExitPlanModal>({
    isOpen: false,
    position: null,
    traderName: '',
  });

  // Fetch decisions for exit plans
  const { data: allDecisions } = useSWR(
    traderIds.length > 0 ? `competition-decisions-${traderIds.sort().join(',')}` : null,
    async () => {
      const promises = traderIds.map(async (traderId) => {
        try {
          const records = await api.getLatestDecisions(traderId);
          return { traderId, records };
        } catch (err) {
          console.error(`Failed to fetch decisions for ${traderId}:`, err);
          return { traderId, records: [] };
        }
      });
      return Promise.all(promises);
    }
  );

  // Parse exit plan from decision JSON
  const getExitPlanForPosition = (traderId: string, symbol: string) => {
    const traderDecision = allDecisions?.find(d => d.traderId === traderId);
    if (!traderDecision) return { stopLoss: undefined, takeProfit: undefined };

    // Look through latest decision records to find the most recent one with exit plan for this symbol
    for (const record of traderDecision.records) {
      try {
        const decisionJson = record.decision_json;
        if (!decisionJson) continue;

        const decisions = JSON.parse(decisionJson);
        const matchingDecision = decisions.find((d: any) => d.symbol === symbol && (d.stop_loss || d.take_profit));
        
        if (matchingDecision) {
          return {
            stopLoss: matchingDecision.stop_loss,
            takeProfit: matchingDecision.take_profit,
          };
        }
      } catch (err) {
        console.error('Failed to parse decision JSON:', err);
      }
    }

    return { stopLoss: undefined, takeProfit: undefined };
  };

  // Fetch positions for all traders
  const { data: allPositions, isLoading } = useSWR(
    traderIds.length > 0 ? `competition-positions-${traderIds.sort().join(',')}` : null,
    async () => {
      const promises = traderIds.map(async (traderId) => {
        try {
          const positions = await api.getPositions(traderId);
          const account = await api.getAccount(traderId);
          
          const trader = traders.find(t => t.trader_id === traderId);
          const totalUnrealizedPnL = positions.reduce((sum, pos) => sum + pos.unrealized_pnl, 0);
          
          return {
            trader_id: traderId,
            trader_name: trader?.trader_name || traderId,
            ai_model: trader?.ai_model || 'UNKNOWN',
            positions,
            totalUnrealizedPnL,
            availableBalance: account?.available_balance || 0,
          };
        } catch (err) {
          console.error(`Failed to fetch positions for ${traderId}:`, err);
          return null;
        }
      });

      const results = await Promise.all(promises);
      return results.filter((item): item is TraderPositions => item !== null);
    },
    {
      refreshInterval: 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  );

  // Get model icon
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

  // Get model color
  const getModelColor = (traderId: string) => {
    return getTraderColor(traders, traderId);
  };

  if (isLoading) {
    return (
      <div className="space-y-4">
        {[1, 2].map((i) => (
          <div key={i} className="binance-card p-6 animate-pulse">
            <div className="skeleton h-8 w-48 mb-4"></div>
            <div className="skeleton h-32 w-full"></div>
          </div>
        ))}
      </div>
    );
  }

  if (!allPositions || allPositions.length === 0) {
    return (
      <div className="binance-card p-6">
        <div className="text-center py-12">
          <div className="text-6xl mb-4 opacity-30">📊</div>
          <div className="text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
            {t('noActivePositionsCompetition', language)}
          </div>
        </div>
      </div>
    );
  }

  // Filter traders with positions
  const tradersWithPositions = allPositions.filter(t => t.positions.length > 0);

  // Apply model filter
  const filteredTraders = selectedModel === 'ALL' 
    ? tradersWithPositions 
    : tradersWithPositions.filter(t => t.ai_model.toUpperCase() === selectedModel);

  if (filteredTraders.length === 0) {
    return (
      <div className="binance-card p-6">
        <div className="text-center py-12">
          <div className="text-6xl mb-4 opacity-30">📊</div>
          <div className="text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
            {t('noActivePositionsCompetition', language)}
          </div>
        </div>
      </div>
    );
  }

  return (
    <>
      {/* Exit Plan Modal */}
      {modal.isOpen && modal.position && (
        <div
          className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
          onClick={() => setModal({ isOpen: false, position: null, traderName: '' })}
        >
          <div
            className="binance-card p-6 max-w-md w-full"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-bold" style={{ color: '#EAECEF' }}>
                {t('exitPlan', language)}
              </h3>
              <button
                onClick={() => setModal({ isOpen: false, position: null, traderName: '' })}
                className="text-2xl font-bold hover:opacity-70 transition-opacity"
                style={{ color: '#848E9C' }}
              >
                ×
              </button>
            </div>

            <div className="space-y-4">
              <div>
                <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                  {modal.position.symbol}
                </div>
                <div className="text-sm" style={{ color: '#EAECEF' }}>
                  {modal.traderName}
                </div>
              </div>

              {modal.stopLoss && (
                <div className="border-t border-b py-3" style={{ borderColor: '#2B3139' }}>
                  <div className="text-sm font-semibold mb-1" style={{ color: '#F6465D' }}>
                    Stop Loss
                  </div>
                  <div className="text-lg font-bold mono" style={{ color: '#F6465D' }}>
                    ${modal.stopLoss.toFixed(2)}
                  </div>
                </div>
              )}

              {modal.takeProfit && (
                <div className="border-b py-3" style={{ borderColor: '#2B3139' }}>
                  <div className="text-sm font-semibold mb-1" style={{ color: '#0ECB81' }}>
                    Take Profit
                  </div>
                  <div className="text-lg font-bold mono" style={{ color: '#0ECB81' }}>
                    ${modal.takeProfit.toFixed(2)}
                  </div>
                </div>
              )}

              {!modal.stopLoss && !modal.takeProfit && (
                <div className="text-center py-8">
                  <div className="text-4xl mb-2 opacity-30">📝</div>
                  <div className="text-sm" style={{ color: '#848E9C' }}>
                    No exit plan data available
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Model Filter */}
      <div className="mb-4">
        <select
          value={selectedModel}
          onChange={(e) => setSelectedModel(e.target.value)}
          className="w-full md:w-auto px-4 py-2 rounded font-semibold"
          style={{
            background: '#1E2329',
            color: '#EAECEF',
            border: '1px solid #2B3139',
          }}
        >
          <option value="ALL">FILTER: ALL MODELS</option>
          {Array.from(new Set(tradersWithPositions.map(t => t.ai_model.toUpperCase()))).map(model => (
            <option key={model} value={model}>{model}</option>
          ))}
        </select>
      </div>

      <div className="space-y-4">
        {filteredTraders.map((traderData) => {
        const traderColor = getModelColor(traderData.trader_id);
        const isPositive = traderData.totalUnrealizedPnL >= 0;

        return (
          <div
            key={traderData.trader_id}
            className="binance-card p-5"
            style={{
              border: `1px solid ${traderColor}30`,
              background: '#1E2329',
            }}
          >
            {/* Model Header */}
            <div className="flex items-center justify-between mb-4 pb-3 border-b" style={{ borderColor: `${traderColor}20` }}>
              <div className="flex items-center gap-3">
                <div
                  className="w-10 h-10 rounded-xl flex items-center justify-center text-xl font-bold shadow-lg"
                  style={{
                    background: `linear-gradient(135deg, ${traderColor} 0%, ${traderColor}dd 100%)`,
                    border: `2px solid ${traderColor}`,
                    boxShadow: `0 4px 12px ${traderColor}40`
                  }}
                >
                  {getModelIcon(traderData.ai_model)}
                </div>
                <div>
                  <div className="font-bold text-base" style={{ color: traderColor }}>
                    {traderData.ai_model.toUpperCase()}
                  </div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {traderData.trader_name}
                  </div>
                </div>
              </div>
              <div className="text-right">
                <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                  {t('totalUnrealizedPnL', language)}
                </div>
                <div
                  className="text-lg font-bold mono"
                  style={{ color: isPositive ? '#0ECB81' : '#F6465D' }}
                >
                  {isPositive ? '+' : ''}${traderData.totalUnrealizedPnL.toFixed(2)}
                </div>
              </div>
            </div>

            {/* Positions Table */}
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b" style={{ borderColor: '#2B3139' }}>
                    <th className="pb-2 text-left font-semibold" style={{ color: '#848E9C' }}>
                      {t('side', language)}
                    </th>
                    <th className="pb-2 text-left font-semibold" style={{ color: '#848E9C' }}>
                      {t('symbol', language)}
                    </th>
                    <th className="pb-2 text-left font-semibold" style={{ color: '#848E9C' }}>
                      {t('leverage', language)}
                    </th>
                    <th className="pb-2 text-left font-semibold" style={{ color: '#848E9C' }}>
                      {t('notional', language)}
                    </th>
                    <th className="pb-2 text-left font-semibold" style={{ color: '#848E9C' }}>
                      {t('exitPlan', language)}
                    </th>
                    <th className="pb-2 text-right font-semibold" style={{ color: '#848E9C' }}>
                      {t('unrealizedPnLShort', language)}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {traderData.positions.map((pos, i) => {
                    const notional = pos.quantity * pos.mark_price;
                    const isPosPositive = pos.unrealized_pnl >= 0;

                    return (
                      <tr
                        key={i}
                        className="border-b last:border-0"
                        style={{ borderColor: '#2B3139' }}
                      >
                        <td className="py-3">
                          <span
                            className="px-2 py-1 rounded text-xs font-bold"
                            style={
                              pos.side === 'long'
                                ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                                : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
                            }
                          >
                            {t(pos.side.toUpperCase(), language)}
                          </span>
                        </td>
                        <td className="py-3">
                          <div className="flex items-center gap-2 font-mono font-semibold" style={{ color: '#EAECEF' }}>
                            {(() => {
                              const icon = getCoinIcon(pos.symbol);
                              const isXRP = pos.symbol.toUpperCase().includes('XRP');
                              return icon.type === 'svg' ? (
                                <div 
                                  className="flex-shrink-0 relative"
                                  style={
                                    isXRP
                                      ? {
                                          background: 'rgba(255, 255, 255, 0.15)',
                                          borderRadius: '50%',
                                          padding: '2px',
                                          width: '24px',
                                          height: '24px',
                                          display: 'flex',
                                          alignItems: 'center',
                                          justifyContent: 'center',
                                        }
                                      : {}
                                  }
                                >
                                  <img 
                                    src={icon.value} 
                                    alt={pos.symbol}
                                    className="w-5 h-5 flex-shrink-0"
                                    style={
                                      isXRP
                                        ? {
                                            filter: 'brightness(1.2) contrast(1.1)',
                                          }
                                        : {}
                                    }
                                    onError={(e) => {
                                      // Fallback to emoji if image fails to load
                                      const target = e.target as HTMLImageElement;
                                      target.style.display = 'none';
                                      const fallback = document.createElement('span');
                                      fallback.className = 'text-lg';
                                      fallback.textContent = '◈';
                                      target.parentNode?.insertBefore(fallback, target);
                                    }}
                                  />
                                </div>
                              ) : (
                                <span className="text-lg">{icon.value}</span>
                              );
                            })()}
                            {pos.symbol}
                          </div>
                        </td>
                        <td className="py-3 font-mono" style={{ color: '#F0B90B' }}>
                          {pos.leverage}x
                        </td>
                        <td className="py-3 font-mono font-bold" style={{ color: '#0ECB81' }}>
                          ${notional.toFixed(0)}
                        </td>
                        <td className="py-3">
                          <button
                            onClick={() => {
                              const exitPlan = getExitPlanForPosition(traderData.trader_id, pos.symbol);
                              setModal({
                                isOpen: true,
                                position: pos,
                                traderName: traderData.trader_name,
                                stopLoss: exitPlan.stopLoss,
                                takeProfit: exitPlan.takeProfit,
                              });
                            }}
                            className="px-2 py-1 rounded text-xs font-semibold transition-colors"
                            style={{
                              background: 'rgba(96, 165, 250, 0.1)',
                              color: '#60a5fa',
                              border: '1px solid rgba(96, 165, 250, 0.2)'
                            }}
                            onMouseEnter={(e) => {
                              e.currentTarget.style.background = 'rgba(96, 165, 250, 0.2)';
                            }}
                            onMouseLeave={(e) => {
                              e.currentTarget.style.background = 'rgba(96, 165, 250, 0.1)';
                            }}
                          >
                            {t('view', language)}
                          </button>
                        </td>
                        <td className="py-3 text-right">
                          <span
                            className="font-mono font-bold"
                            style={{ color: isPosPositive ? '#0ECB81' : '#F6465D' }}
                          >
                            {isPosPositive ? '+' : ''}${pos.unrealized_pnl.toFixed(2)}
                          </span>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            {/* Available Cash */}
            <div className="mt-4 pt-3 border-t flex justify-between items-center" style={{ borderColor: '#2B3139' }}>
              <span className="text-xs font-semibold uppercase" style={{ color: '#848E9C' }}>
                {t('availableCash', language)}:
              </span>
              <span className="text-sm font-bold mono" style={{ color: '#0ECB81' }}>
                ${traderData.availableBalance.toFixed(2)}
              </span>
            </div>
          </div>
        );
      })}
      </div>
    </>
  );
}

