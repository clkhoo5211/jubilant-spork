import { useState } from 'react';
import useSWR from 'swr';
import { api } from '../lib/api';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';
import { getTraderColor } from '../utils/traderColors';
import type { DecisionRecord } from '../types';

interface CompetitionModelChatProps {
  traderIds: string[];
  maxMessages?: number;
  autoRefresh?: boolean;
}

interface AggregatedMessage {
  trader_id: string;
  trader_name: string;
  ai_model: string;
  decision: DecisionRecord;
}

export function CompetitionModelChat({ 
  traderIds, 
  maxMessages = 50, 
  autoRefresh = true 
}: CompetitionModelChatProps) {
  const { language } = useLanguage();
  const [expandedMessages, setExpandedMessages] = useState<Set<string>>(new Set());
  const [selectedModel, setSelectedModel] = useState<string>('all');
  const [expandedModels, setExpandedModels] = useState<Set<string>>(new Set());

  // Fetch decisions for all traders
  const { data: aggregatedDecisions, error, isLoading } = useSWR(
    traderIds.length > 0 ? `competition-decisions-${traderIds.sort().join(',')}` : null,
    async () => {
      const promises = traderIds.map(async (traderId) => {
        try {
          const decisions = await api.getLatestDecisions(traderId);
          return decisions.map(decision => ({
            trader_id: traderId,
            decision
          }));
        } catch (err) {
          console.error(`Failed to fetch decisions for ${traderId}:`, err);
          return [];
        }
      });

      const results = await Promise.all(promises);
      return results.flat();
    },
    {
      refreshInterval: autoRefresh ? 30000 : 0,
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  );

  // Get trader info from competition data
  const { data: competition } = useSWR('competition', api.getCompetition);

  const toggleMessage = (key: string) => {
    setExpandedMessages((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(key)) {
        newSet.delete(key);
      } else {
        newSet.add(key);
      }
      return newSet;
    });
  };

  if (isLoading) {
    return (
      <div className="binance-card p-6">
        <div className="flex items-center gap-3" style={{ color: '#848E9C' }}>
          <div className="spinner"></div>
          <span>{t('loading', language)}</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="binance-card p-6">
        <div className="flex items-center gap-2 p-4 rounded" style={{ background: 'rgba(246, 70, 93, 0.1)', border: '1px solid rgba(246, 70, 93, 0.2)' }}>
          <span className="text-xl">⚠️</span>
          <div>
            <div className="font-semibold" style={{ color: '#F6465D' }}>{t('loadingError', language)}</div>
            <div className="text-sm" style={{ color: '#848E9C' }}>{error.message}</div>
          </div>
        </div>
      </div>
    );
  }

  if (!aggregatedDecisions || aggregatedDecisions.length === 0) {
    return (
      <div className="binance-card p-6">
        <div className="text-center py-12">
          <div className="text-6xl mb-4 opacity-30">💬</div>
          <div className="text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
            {t('noChatMessagesYet', language)}
          </div>
          <div className="text-sm" style={{ color: '#848E9C' }}>
            {t('chatWillAppear', language)}
          </div>
        </div>
      </div>
    );
  }

  // Build aggregated messages with trader info
  const messages: AggregatedMessage[] = aggregatedDecisions
    .filter((item: any) => item?.decision && item.decision.timestamp) // Filter out invalid decisions
    .map((item: any) => {
      const trader = competition?.traders?.find(t => t.trader_id === item.trader_id);
      return {
        trader_id: item.trader_id,
        trader_name: trader?.trader_name || item.trader_id,
        ai_model: trader?.ai_model || 'UNKNOWN',
        decision: item.decision
      };
    });

  // Sort ALL messages by timestamp (newest first) - this ensures chronological order
  messages.sort((a, b) => {
    const timestampA = a.decision?.timestamp ? new Date(a.decision.timestamp).getTime() : 0;
    const timestampB = b.decision?.timestamp ? new Date(b.decision.timestamp).getTime() : 0;
    return timestampB - timestampA;
  });

  // Filter by selected model
  const modelFilteredMessages = selectedModel === 'all' 
    ? messages
    : messages.filter(m => m.ai_model.toLowerCase() === selectedModel.toLowerCase());

  // Count messages per model to determine what to show
  const DEFAULT_MESSAGES_PER_MODEL = 5;
  const messagesByModel = new Map<string, AggregatedMessage[]>();
  const modelCounts = new Map<string, number>();
  
  modelFilteredMessages.forEach(msg => {
    const modelKey = `${msg.ai_model}|${msg.trader_id}`;
    if (!messagesByModel.has(modelKey)) {
      messagesByModel.set(modelKey, []);
      modelCounts.set(modelKey, 0);
    }
    messagesByModel.get(modelKey)!.push(msg);
    modelCounts.set(modelKey, (modelCounts.get(modelKey) || 0) + 1);
  });

  // Build display list: maintain chronological order but limit per model
  const displayItems: Array<{
    type: 'message' | 'showMore';
    message?: AggregatedMessage;
    modelKey?: string;
    modelName?: string;
    traderId?: string;
    remainingCount?: number;
  }> = [];

  const messagesShownPerModel = new Map<string, number>();
  
  modelFilteredMessages.forEach(msg => {
    const modelKey = `${msg.ai_model}|${msg.trader_id}`;
    const shown = messagesShownPerModel.get(modelKey) || 0;
    const isExpanded = expandedModels.has(modelKey);
    const maxToShow = isExpanded ? (modelCounts.get(modelKey) || 0) : DEFAULT_MESSAGES_PER_MODEL;
    
    if (shown < maxToShow) {
      displayItems.push({ type: 'message', message: msg });
      messagesShownPerModel.set(modelKey, shown + 1);
      
      // Check if we should add Show More/Less button after this message
      const totalCount = modelCounts.get(modelKey) || 0;
      const isLastMessage = (shown + 1) === maxToShow;
      
      if (isLastMessage && totalCount > DEFAULT_MESSAGES_PER_MODEL) {
        if (!isExpanded) {
          // Show More button after 5th message
          displayItems.push({
            type: 'showMore',
            modelKey: modelKey,
            modelName: msg.ai_model,
            traderId: msg.trader_id,
            remainingCount: totalCount - DEFAULT_MESSAGES_PER_MODEL
          });
        } else {
          // Show Less button after all messages when expanded
          displayItems.push({
            type: 'showMore',
            modelKey: modelKey,
            modelName: msg.ai_model,
            traderId: msg.trader_id,
            remainingCount: totalCount - DEFAULT_MESSAGES_PER_MODEL
          });
        }
      }
    }
  });

  // Get unique models for filter
  const uniqueModels = Array.from(new Set(messages.map(m => m.ai_model)));

  const toggleModelExpansion = (modelKey: string) => {
    setExpandedModels((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(modelKey)) {
        newSet.delete(modelKey);
      } else {
        newSet.add(modelKey);
      }
      return newSet;
    });
  };

  return (
    <div>
      {/* Filter Bar */}
      <div className="flex items-center gap-3 mb-4">
        <span className="text-xs font-semibold uppercase tracking-wider" style={{ color: '#848E9C' }}>
          {t('filter', language)}:
        </span>
        <select
          value={selectedModel}
          onChange={(e) => setSelectedModel(e.target.value)}
          className="rounded px-3 py-2 text-sm font-medium cursor-pointer transition-colors"
          style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
        >
          <option value="all">{t('allModels', language)}</option>
          {uniqueModels.map(model => (
            <option key={model} value={model.toLowerCase()}>{model.toUpperCase()}</option>
          ))}
        </select>
      </div>

      {/* Messages Feed - Chronologically Sorted */}
      <div className="space-y-3">
        {displayItems.map((item, idx) => {
          if (item.type === 'showMore') {
            // Show More button
            const modelKey = item.modelKey!;
            const isModelExpanded = expandedModels.has(modelKey);
            const trader = competition?.traders?.find(t => t.trader_id === item.traderId);
            const traderColor = competition?.traders && item.traderId
              ? getTraderColor(competition.traders, item.traderId)
              : '#848E9C';

            return (
              <div key={`showMore-${modelKey}-${idx}`} className="flex justify-center py-3 my-2">
                <button
                  onClick={() => toggleModelExpansion(modelKey)}
                  className="px-6 py-2.5 rounded-lg text-sm font-bold transition-all duration-200 hover:scale-105 shadow-lg"
                  style={{
                    background: `${traderColor}20`,
                    color: traderColor,
                    border: `2px solid ${traderColor}60`,
                    boxShadow: `0 4px 12px ${traderColor}30`
                  }}
                >
                  {isModelExpanded 
                    ? `▲ ${t('showLess', language)} (${item.remainingCount} ${t('messages', language)})` 
                    : `▼ ${t('showMore', language)} (${item.remainingCount} ${t('messages', language)})`
                  }
                </button>
              </div>
            );
          }

          // Message item
          const msg = item.message!;
          const key = `${msg.trader_id}-${msg.decision.cycle_number}-${msg.decision.timestamp}`;
          const isExpanded = expandedMessages.has(key);
          
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

          // Use the same color system as performance comparison chart
          const traderColor = competition?.traders 
            ? getTraderColor(competition.traders, msg.trader_id)
            : '#848E9C';

          return (
            <div
              key={key}
              className="rounded transition-all duration-300 hover:translate-y-[-2px]"
              style={{
                border: `1px solid ${traderColor}40`,
                background: isExpanded ? '#0B0E11' : '#1E2329',
                boxShadow: isExpanded 
                  ? `0 4px 12px ${traderColor}20` 
                  : `0 2px 8px ${traderColor}10`
              }}
            >
              {/* Message Header */}
              <button
                onClick={() => toggleMessage(key)}
                className="w-full p-4 flex items-center justify-between transition-colors hover:bg-opacity-50"
                style={{ background: `${traderColor}10` }}
              >
                <div className="flex items-center gap-3 flex-1">
                  <div className="flex items-center gap-2 flex-shrink-0">
                    <div 
                      className="w-10 h-10 rounded-full flex items-center justify-center text-lg font-bold shadow-lg"
                      style={{
                        background: `linear-gradient(135deg, ${traderColor} 0%, ${traderColor}dd 100%)`,
                        border: `2px solid ${traderColor}`,
                        boxShadow: `0 4px 12px ${traderColor}40`
                      }}
                    >
                      {getModelIcon(msg.ai_model)}
                    </div>
                  </div>
                  <div className="text-left flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span 
                        className="font-bold text-base truncate"
                        style={{ color: traderColor }}
                      >
                        {msg.ai_model.toUpperCase()}
                      </span>
                      {selectedModel === 'all' && (
                        <span className="text-xs px-2 py-0.5 rounded font-semibold" style={{
                          background: `${traderColor}20`,
                          color: traderColor,
                          border: `1px solid ${traderColor}40`
                        }}>
                          {msg.trader_name}
                        </span>
                      )}
                    </div>
                    <div className="text-xs mt-1 flex items-center gap-2 flex-wrap">
                      <span style={{ color: '#848E9C' }}>
                        {t('cycle', language)} #{msg.decision.cycle_number}
                      </span>
                      <span style={{ color: '#5E6673' }}>•</span>
                      <span style={{ color: '#848E9C' }}>
                        {new Date(msg.decision.timestamp).toLocaleString()}
                      </span>
                      {msg.decision.decisions && msg.decision.decisions.length > 0 && (
                        <>
                          <span style={{ color: '#5E6673' }}>•</span>
                          <span style={{ color: '#848E9C' }}>
                            {msg.decision.decisions.length} {t('actions', language)}
                          </span>
                        </>
                      )}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2 flex-shrink-0">
                  <div
                    className="px-3 py-1 rounded text-xs font-bold"
                    style={msg.decision.success
                      ? { background: 'rgba(14, 203, 129, 0.15)', color: '#0ECB81', border: '1px solid rgba(14, 203, 129, 0.3)' }
                      : { background: 'rgba(246, 70, 93, 0.15)', color: '#F6465D', border: '1px solid rgba(246, 70, 93, 0.3)' }
                    }
                  >
                    {t(msg.decision.success ? 'success' : 'failed', language)}
                  </div>
                  <span className="text-lg" style={{ color: traderColor }}>
                    {isExpanded ? '▲' : '▼'}
                  </span>
                </div>
              </button>

              {/* Message Content - Collapsible */}
              {isExpanded && (
                <div className="p-4 space-y-4 animate-slide-down border-t" style={{ borderColor: `${traderColor}20` }}>
                  {/* Chain of Thought */}
                  {msg.decision.cot_trace && (
                    <div>
                      <div className="flex items-center gap-2 mb-2">
                        <span className="text-sm font-bold" style={{ color: '#F0B90B' }}>
                          🧠 {t('aiThinking', language)}
                        </span>
                      </div>
                      <div className="rounded p-3 text-sm font-mono whitespace-pre-wrap max-h-64 overflow-y-auto" style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF'
                      }}>
                        {msg.decision.cot_trace}
                      </div>
                    </div>
                  )}

                  {/* Decisions Actions */}
                  {msg.decision.decisions && msg.decision.decisions.length > 0 && (
                    <div>
                      <div className="flex items-center gap-2 mb-2">
                        <span className="text-sm font-bold" style={{ color: '#60a5fa' }}>
                          📋 {t('decisionActions', language)}
                        </span>
                      </div>
                      <div className="space-y-2">
                        {msg.decision.decisions.map((action: any, j: number) => (
                          <div
                            key={j}
                            className="flex items-center gap-2 p-2 rounded text-sm"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                          >
                            <span className="font-mono font-bold" style={{ color: '#EAECEF' }}>
                              {action.symbol}
                            </span>
                            <span
                              className="px-2 py-1 rounded text-xs font-bold"
                              style={action.action.includes('open')
                                ? { background: 'rgba(96, 165, 250, 0.1)', color: '#60a5fa' }
                                : { background: 'rgba(240, 185, 11, 0.1)', color: '#F0B90B' }
                              }
                            >
                              {action.action}
                            </span>
                            {action.leverage > 0 && (
                              <span style={{ color: '#F0B90B' }}>{action.leverage}x</span>
                            )}
                            {action.price > 0 && (
                              <span className="font-mono text-xs" style={{ color: '#848E9C' }}>
                                @{action.price.toFixed(4)}
                              </span>
                            )}
                            <span style={{ color: action.success ? '#0ECB81' : '#F6465D' }}>
                              {action.success ? '✓' : '✗'}
                            </span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Account State Summary */}
                  {msg.decision.account_state && (
                    <div>
                      <div className="flex items-center gap-2 mb-2">
                        <span className="text-sm font-bold" style={{ color: '#94A3B8' }}>
                          📊 {t('accountState', language)}
                        </span>
                      </div>
                      <div className="grid grid-cols-2 gap-2 p-3 rounded text-xs" style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#848E9C'
                      }}>
                        <span>
                          {t('equity', language)}: {msg.decision.account_state.total_balance.toFixed(2)} USDT
                        </span>
                        <span>
                          {t('availableBalance', language)}: {msg.decision.account_state.available_balance.toFixed(2)} USDT
                        </span>
                        <span>
                          {t('margin', language)}: {msg.decision.account_state.margin_used_pct.toFixed(1)}%
                        </span>
                        <span>
                          {t('positions', language)}: {msg.decision.account_state.position_count}
                        </span>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Info Message */}
      {displayItems.length > 0 && (
        <div className="text-center mt-4 text-xs" style={{ color: '#848E9C' }}>
          {language === 'en' 
            ? `Messages sorted by time (newest first). Showing latest ${DEFAULT_MESSAGES_PER_MODEL} per model. Click "Show More" to view full history.`
            : `消息按时间排序（最新在前）。默认显示每个模型最新 ${DEFAULT_MESSAGES_PER_MODEL} 条消息。点击"显示更多"查看完整历史。`
          }
        </div>
      )}
    </div>
  );
}

