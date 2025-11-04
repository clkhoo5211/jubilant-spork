#!/bin/bash
# Generate config.json from environment variables
# This allows deployment to Render/Heroku without committing sensitive keys

set -e

CONFIG_FILE="${1:-config.json}"

echo "🔧 Generating config.json from environment variables..."

# Start with base config structure
cat > "$CONFIG_FILE" << 'EOF'
{
  "traders": [
EOF

# Track if we've added any traders (for comma handling)
TRADER_COUNT=0

# Trader 1 configuration
if [ -n "$TRADER_1_ENABLED" ]; then
  if [ $TRADER_COUNT -gt 0 ]; then
    echo "," >> "$CONFIG_FILE"
  fi
  TRADER_COUNT=$((TRADER_COUNT + 1))
  
  cat >> "$CONFIG_FILE" << EOF
    {
      "id": "trader_1",
      "name": "${TRADER_1_NAME:-Gemini 2.5 Flash Stable}",
      "enabled": ${TRADER_1_ENABLED:-true},
      "ai_model": "${TRADER_1_AI_MODEL:-custom}",
      "exchange": "${TRADER_1_EXCHANGE:-binance}",
EOF

  # Exchange-specific keys for trader 1
  if [ "${TRADER_1_EXCHANGE:-binance}" = "binance" ]; then
    cat >> "$CONFIG_FILE" << EOF
      "binance_api_key": "${TRADER_1_BINANCE_API_KEY}",
      "binance_secret_key": "${TRADER_1_BINANCE_SECRET_KEY}",
      "binance_testnet": ${TRADER_1_BINANCE_TESTNET:-true},
EOF
  elif [ "${TRADER_1_EXCHANGE}" = "gateio" ]; then
    cat >> "$CONFIG_FILE" << EOF
      "gateio_api_key": "${TRADER_1_GATEIO_API_KEY}",
      "gateio_secret_key": "${TRADER_1_GATEIO_SECRET_KEY}",
      "gateio_testnet": ${TRADER_1_GATEIO_TESTNET:-true},
EOF
  fi

  cat >> "$CONFIG_FILE" << EOF
      "custom_api_url": "${TRADER_1_CUSTOM_API_URL:-https://generativelanguage.googleapis.com/v1beta}",
      "custom_api_key": "${TRADER_1_CUSTOM_API_KEY}",
      "custom_model_name": "${TRADER_1_CUSTOM_MODEL_NAME:-gemini-2.5-flash}",
      "initial_balance": ${TRADER_1_INITIAL_BALANCE:-4618.06},
      "scan_interval_minutes": ${TRADER_1_SCAN_INTERVAL:-3}
EOF
  # Add system_prompt_template if set
  if [ -n "${TRADER_1_SYSTEM_PROMPT_TEMPLATE:-}" ]; then
    echo "      ," >> "$CONFIG_FILE"
    echo "      \"system_prompt_template\": \"${TRADER_1_SYSTEM_PROMPT_TEMPLATE}\"" >> "$CONFIG_FILE"
  fi
  cat >> "$CONFIG_FILE" << 'EOF'
    }
EOF
fi

# Trader 2 configuration
if [ -n "$TRADER_2_ENABLED" ]; then
  if [ $TRADER_COUNT -gt 0 ]; then
    echo "," >> "$CONFIG_FILE"
  fi
  TRADER_COUNT=$((TRADER_COUNT + 1))
  
  cat >> "$CONFIG_FILE" << EOF
    {
      "id": "trader_2",
      "name": "${TRADER_2_NAME:-Gemini 2.5 Flash-Lite Fast}",
      "enabled": ${TRADER_2_ENABLED:-true},
      "ai_model": "${TRADER_2_AI_MODEL:-custom}",
      "exchange": "${TRADER_2_EXCHANGE:-gateio}",
EOF

  if [ "${TRADER_2_EXCHANGE:-gateio}" = "binance" ]; then
    cat >> "$CONFIG_FILE" << EOF
      "binance_api_key": "${TRADER_2_BINANCE_API_KEY}",
      "binance_secret_key": "${TRADER_2_BINANCE_SECRET_KEY}",
      "binance_testnet": ${TRADER_2_BINANCE_TESTNET:-true},
EOF
  elif [ "${TRADER_2_EXCHANGE:-gateio}" = "gateio" ]; then
    cat >> "$CONFIG_FILE" << EOF
      "gateio_api_key": "${TRADER_2_GATEIO_API_KEY}",
      "gateio_secret_key": "${TRADER_2_GATEIO_SECRET_KEY}",
      "gateio_testnet": ${TRADER_2_GATEIO_TESTNET:-true},
EOF
  fi

  cat >> "$CONFIG_FILE" << EOF
      "custom_api_url": "${TRADER_2_CUSTOM_API_URL:-https://generativelanguage.googleapis.com/v1beta}",
      "custom_api_key": "${TRADER_2_CUSTOM_API_KEY}",
      "custom_model_name": "${TRADER_2_CUSTOM_MODEL_NAME:-gemini-2.5-flash-lite}",
      "initial_balance": ${TRADER_2_INITIAL_BALANCE:-2021.33},
      "scan_interval_minutes": ${TRADER_2_SCAN_INTERVAL:-3}
EOF
  # Add system_prompt_template if set
  if [ -n "${TRADER_2_SYSTEM_PROMPT_TEMPLATE:-}" ]; then
    echo "      ," >> "$CONFIG_FILE"
    echo "      \"system_prompt_template\": \"${TRADER_2_SYSTEM_PROMPT_TEMPLATE}\"" >> "$CONFIG_FILE"
  fi
  cat >> "$CONFIG_FILE" << 'EOF'
    }
EOF
fi

# Trader 3 configuration
if [ -n "$TRADER_3_ENABLED" ]; then
  if [ $TRADER_COUNT -gt 0 ]; then
    echo "," >> "$CONFIG_FILE"
  fi
  TRADER_COUNT=$((TRADER_COUNT + 1))
  
  cat >> "$CONFIG_FILE" << EOF
    {
      "id": "trader_3",
      "name": "${TRADER_3_NAME:-Gemini Flash Latest Auto}",
      "enabled": ${TRADER_3_ENABLED:-true},
      "ai_model": "${TRADER_3_AI_MODEL:-custom}",
      "exchange": "${TRADER_3_EXCHANGE:-binance}",
EOF

  if [ "${TRADER_3_EXCHANGE:-binance}" = "binance" ]; then
    cat >> "$CONFIG_FILE" << EOF
      "binance_api_key": "${TRADER_3_BINANCE_API_KEY}",
      "binance_secret_key": "${TRADER_3_BINANCE_SECRET_KEY}",
      "binance_testnet": ${TRADER_3_BINANCE_TESTNET:-true},
EOF
  elif [ "${TRADER_3_EXCHANGE}" = "gateio" ]; then
    cat >> "$CONFIG_FILE" << EOF
      "gateio_api_key": "${TRADER_3_GATEIO_API_KEY}",
      "gateio_secret_key": "${TRADER_3_GATEIO_SECRET_KEY}",
      "gateio_testnet": ${TRADER_3_GATEIO_TESTNET:-true},
EOF
  fi

  cat >> "$CONFIG_FILE" << EOF
      "custom_api_url": "${TRADER_3_CUSTOM_API_URL:-https://generativelanguage.googleapis.com/v1beta}",
      "custom_api_key": "${TRADER_3_CUSTOM_API_KEY}",
      "custom_model_name": "${TRADER_3_CUSTOM_MODEL_NAME:-gemini-flash-latest}",
      "initial_balance": ${TRADER_3_INITIAL_BALANCE:-4918.66},
      "scan_interval_minutes": ${TRADER_3_SCAN_INTERVAL:-3}
EOF
  # Add system_prompt_template if set
  if [ -n "${TRADER_3_SYSTEM_PROMPT_TEMPLATE:-}" ]; then
    echo "      ," >> "$CONFIG_FILE"
    echo "      \"system_prompt_template\": \"${TRADER_3_SYSTEM_PROMPT_TEMPLATE}\"" >> "$CONFIG_FILE"
  fi
  cat >> "$CONFIG_FILE" << 'EOF'
    }
EOF
fi

# Close traders array and add global config
# Note: If no traders were added, we still have a valid JSON with empty array
cat >> "$CONFIG_FILE" << 'EOF'
  ],
  "leverage": {
    "btc_eth_leverage": 5,
    "altcoin_leverage": 5
  },
  "use_default_coins": true,
  "default_coins": ["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT"],
  "coin_pool_api_url": "",
  "oi_top_api_url": "",
  "api_server_port": 8080,
  "max_daily_loss": 10.0,
  "max_drawdown": 20.0,
  "stop_trading_minutes": 60,
  "market_data_provider": "binance",
  "position_size": {
    "min_position_size_usd": 0,
    "max_position_size_usd": 150,
    "max_margin_usage_pct": 80.0,
    "max_position_size_mult": 1.5,
    "safety_buffer_pct": 5.0,
    "check_available_before_open": true
  },
  "web_username": "admin",
  "web_password": "admin123"
}
EOF

# Replace default values with environment variables if set
if [ -n "${LEVERAGE_BTC_ETH:-}" ]; then
  sed -i.bak "s/\"btc_eth_leverage\": 5/\"btc_eth_leverage\": $LEVERAGE_BTC_ETH/" "$CONFIG_FILE" && rm -f "$CONFIG_FILE.bak" 2>/dev/null || true
fi
if [ -n "${LEVERAGE_ALTCOIN:-}" ]; then
  sed -i.bak "s/\"altcoin_leverage\": 5/\"altcoin_leverage\": $LEVERAGE_ALTCOIN/" "$CONFIG_FILE" && rm -f "$CONFIG_FILE.bak" 2>/dev/null || true
fi
if [ -n "${API_SERVER_PORT:-}" ]; then
  sed -i.bak "s/\"api_server_port\": 8080/\"api_server_port\": $API_SERVER_PORT/" "$CONFIG_FILE" && rm -f "$CONFIG_FILE.bak" 2>/dev/null || true
fi
if [ -n "${WEB_USERNAME:-}" ]; then
  sed -i.bak "s/\"web_username\": \"admin\"/\"web_username\": \"$WEB_USERNAME\"/" "$CONFIG_FILE" && rm -f "$CONFIG_FILE.bak" 2>/dev/null || true
fi
if [ -n "${WEB_PASSWORD:-}" ]; then
  sed -i.bak "s/\"web_password\": \"admin123\"/\"web_password\": \"$WEB_PASSWORD\"/" "$CONFIG_FILE" && rm -f "$CONFIG_FILE.bak" 2>/dev/null || true
fi
if [ -n "${MARKET_DATA_PROVIDER:-}" ]; then
  sed -i.bak "s/\"market_data_provider\": \"binance\"/\"market_data_provider\": \"$MARKET_DATA_PROVIDER\"/" "$CONFIG_FILE" && rm -f "$CONFIG_FILE.bak" 2>/dev/null || true
else
  # Ensure default is set if not provided
  sed -i.bak "s/\"market_data_provider\": \"\${MARKET_DATA_PROVIDER:-binance}\"/\"market_data_provider\": \"binance\"/" "$CONFIG_FILE" && rm -f "$CONFIG_FILE.bak" 2>/dev/null || true
fi

# Validate JSON
if ! python3 -m json.tool "$CONFIG_FILE" > /dev/null 2>&1 && ! command -v jq > /dev/null 2>&1; then
  echo "⚠️  Warning: Could not validate JSON (python3/jq not available)"
else
  if command -v jq > /dev/null 2>&1; then
    jq . "$CONFIG_FILE" > /dev/null 2>&1 || (echo "❌ JSON validation failed" && exit 1)
  elif command -v python3 > /dev/null 2>&1; then
    python3 -m json.tool "$CONFIG_FILE" > /dev/null 2>&1 || (echo "❌ JSON validation failed" && exit 1)
  fi
fi

echo "✅ Config file generated: $CONFIG_FILE"
