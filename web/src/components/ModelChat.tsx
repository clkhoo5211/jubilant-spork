import { useState } from 'react';
import useSWR from 'swr';
import { api } from '../lib/api';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';
import type { DecisionRecord } from '../types';

interface ModelChatProps {
  traderId?: string;
  maxMessages?: number;
  autoRefresh?: boolean;
  showReasoning?: boolean;
}

export function ModelChat({ 
  traderId, 
  maxMessages = 10, 
  autoRefresh = true,
  showReasoning = true 
}: ModelChatProps) {
  const { language } = useLanguage();
  const [expandedMessages, setExpandedMessages] = useState<Set<number>>(new Set());

  const { data: decisions, error, isLoading } = useSWR<DecisionRecord[]>(
    traderId ? `decisions/latest-${traderId}` : null,
    () => api.getLatestDecisions(traderId),
    {
      refreshInterval: autoRefresh ? 30000 : 0,
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  );

  const toggleMessage = (cycleNumber: number) => {
    setExpandedMessages((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(cycleNumber)) {
        newSet.delete(cycleNumber);
      } else {
        newSet.add(cycleNumber);
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

  if (!decisions || decisions.length === 0) {
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

  // Limit messages and reverse for chronological display
  const displayDecisions = [...decisions].slice(0, maxMessages).reverse();

  return (
    <div>
      {/* Header */}
      <div className="flex items-center gap-3 mb-5 pb-4 border-b" style={{ borderColor: '#2B3139' }}>
        <div className="w-10 h-10 rounded-xl flex items-center justify-center text-xl" style={{
          background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
          boxShadow: '0 4px 14px rgba(99, 102, 241, 0.4)'
        }}>
          💬
        </div>
        <div className="flex-1">
          <h2 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {t('modelChat', language)}
          </h2>
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {t('lastCycles', language, { count: displayDecisions.length })}
          </div>
        </div>
      </div>

      {/* Messages List - Scrollable */}
      <div className="space-y-3 overflow-y-auto" style={{ maxHeight: 'calc(100vh - 300px)' }}>
        {displayDecisions.map((decision) => {
          const isExpanded = expandedMessages.has(decision.cycle_number);
          
          return (
            <div
              key={decision.cycle_number}
              className="rounded transition-all duration-300 hover:translate-y-[-2px]"
              style={{
                border: '1px solid #2B3139',
                background: isExpanded ? '#0B0E11' : '#1E2329',
                boxShadow: isExpanded ? '0 4px 12px rgba(99, 102, 241, 0.15)' : '0 2px 8px rgba(0, 0, 0, 0.3)'
              }}
            >
              {/* Message Header */}
              <button
                onClick={() => toggleMessage(decision.cycle_number)}
                className="w-full p-4 flex items-center justify-between transition-colors hover:bg-opacity-50"
                style={{ background: 'rgba(99, 102, 241, 0.05)' }}
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-lg flex items-center justify-center font-bold" style={{
                    background: decision.success
                      ? 'rgba(14, 203, 129, 0.15)'
                      : 'rgba(246, 70, 93, 0.15)',
                    color: decision.success ? '#0ECB81' : '#F6465D'
                  }}>
                    {decision.success ? '✓' : '✗'}
                  </div>
                  <div className="text-left">
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-sm" style={{ color: '#EAECEF' }}>
                        {t('cycle', language)} #{decision.cycle_number}
                      </span>
                      <span
                        className="px-2 py-0.5 rounded text-xs font-bold"
                        style={decision.success
                          ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                          : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
                        }
                      >
                        {t(decision.success ? 'success' : 'failed', language)}
                      </span>
                    </div>
                    <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                      {new Date(decision.timestamp).toLocaleString()}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {decision.decisions && decision.decisions.length > 0 && (
                    <div className="text-xs px-2 py-1 rounded font-semibold" style={{
                      background: 'rgba(240, 185, 11, 0.1)',
                      color: '#F0B90B',
                      border: '1px solid rgba(240, 185, 11, 0.2)'
                    }}>
                      {decision.decisions.length} {t('actions', language)}
                    </div>
                  )}
                  <span className="text-lg" style={{ color: '#6366F1' }}>
                    {isExpanded ? '▲' : '▼'}
                  </span>
                </div>
              </button>

              {/* Message Content - Collapsible */}
              {isExpanded && (
                <div className="p-4 space-y-4 animate-slide-down">
                  {/* Chain of Thought */}
                  {showReasoning && decision.cot_trace && (
                    <div>
                      <div className="flex items-center gap-2 mb-2">
                        <span className="text-sm font-bold" style={{ color: '#F0B90B' }}>
                          🧠 {t('aiThinking', language)}
                        </span>
                      </div>
                      <div className="rounded p-3 text-sm font-mono whitespace-pre-wrap" style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                        maxHeight: '300px',
                        overflowY: 'auto'
                      }}>
                        {decision.cot_trace}
                      </div>
                    </div>
                  )}

                  {/* Decisions Actions */}
                  {decision.decisions && decision.decisions.length > 0 && (
                    <div>
                      <div className="flex items-center gap-2 mb-2">
                        <span className="text-sm font-bold" style={{ color: '#60a5fa' }}>
                          📋 {t('decisionActions', language)}
                        </span>
                      </div>
                      <div className="space-y-2">
                        {decision.decisions.map((action, idx) => (
                          <div
                            key={idx}
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
                  {decision.account_state && (
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
                          {t('equity', language)}: {decision.account_state.total_balance.toFixed(2)} USDT
                        </span>
                        <span>
                          {t('availableBalance', language)}: {decision.account_state.available_balance.toFixed(2)} USDT
                        </span>
                        <span>
                          {t('margin', language)}: {decision.account_state.margin_used_pct.toFixed(1)}%
                        </span>
                        <span>
                          {t('positions', language)}: {decision.account_state.position_count}
                        </span>
                      </div>
                    </div>
                  )}

                  {/* Input Prompt (Optional - Show if available) */}
                  {decision.input_prompt && (
                    <details className="rounded overflow-hidden" style={{ border: '1px solid #2B3139' }}>
                      <summary className="p-2 cursor-pointer text-sm font-bold" style={{ color: '#60a5fa', background: 'rgba(96, 165, 250, 0.05)' }}>
                        📥 {t('inputPrompt', language)}
                      </summary>
                      <div className="p-3 text-xs font-mono whitespace-pre-wrap" style={{
                        background: '#0B0E11',
                        color: '#EAECEF',
                        maxHeight: '200px',
                        overflowY: 'auto'
                      }}>
                        {decision.input_prompt}
                      </div>
                    </details>
                  )}

                  {/* Error Message (if failed) */}
                  {!decision.success && decision.error_message && (
                    <div className="p-3 rounded text-sm" style={{
                      color: '#F6465D',
                      background: 'rgba(246, 70, 93, 0.1)',
                      border: '1px solid rgba(246, 70, 93, 0.2)'
                    }}>
                      ❌ {decision.error_message}
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

