#!/bin/bash

# List of coins we need (excluding ones we already have from nof1.ai)
COINS_NEEDED=(
  "ada" "cardano"
  "dot" "polkadot"
  "matic" "polygon"
  "link" "chainlink"
  "uni" "uniswap"
  "avax" "avalanche"
  "ltc" "litecoin"
  "bch" "bitcoin-cash"
  "xlm" "stellar"
  "atom" "cosmos"
  "icp" "internet-computer"
  "fil" "filecoin"
  "arb" "arbitrum"
  "op" "optimism"
  "sui" "sui"
  "apt" "aptos"
  "inj" "injective-protocol"
  "sei" "sei-network"
  "tia" "celestia"
)

OUTPUT_DIR="web/public/coins"
mkdir -p "$OUTPUT_DIR"

# Try multiple sources for SVG logos
download_from_github() {
  local coin=$1
  local coin_name=$2
  
  # Try different GitHub repositories
  # 1. cryptologos repo
  local url1="https://raw.githubusercontent.com/spothq/cryptocurrency-icons/master/128/color/${coin}.png"
  
  # 2. crypto-icons repo (if different)
  local url2="https://raw.githubusercontent.com/atomiclabs/cryptocurrency-icons/master/svg/color/${coin}.svg"
  
  # 3. CoinGecko API for coin ID
  echo "Attempting to download ${coin_name} (${coin})..."
  
  # First try direct SVG from crypto-icons repo
  if curl -s -f -o "${OUTPUT_DIR}/${coin}.svg" "$url2" && [ -s "${OUTPUT_DIR}/${coin}.svg" ]; then
    echo "✓ Downloaded ${coin}.svg from crypto-icons"
    return 0
  fi
  
  return 1
}

# Function to download from CoinGecko and convert to SVG
download_and_convert() {
  local coin_symbol=$1
  local coin_id=$2
  
  echo "Downloading ${coin_symbol} from CoinGecko..."
  
  # Try to get coin ID from CoinGecko API first
  if [ -z "$coin_id" ]; then
    coin_id=$(curl -s "https://api.coingecko.com/api/v3/search?query=${coin_symbol}" | grep -o "\"id\":\"[^\"]*${coin_symbol}[^\"]*\"" | head -1 | cut -d'"' -f4)
  fi
  
  if [ -z "$coin_id" ]; then
    echo "⚠ Could not find CoinGecko ID for ${coin_symbol}"
    return 1
  fi
  
  # Download large PNG from CoinGecko
  local png_url="https://assets.coingecko.com/coins/images/${coin_id}/large/${coin_symbol}.png"
  local temp_png="${OUTPUT_DIR}/${coin_symbol}_temp.png"
  
  if curl -s -f -o "$temp_png" "$png_url" && [ -s "$temp_png" ]; then
    echo "  Downloaded PNG for ${coin_symbol}"
    # Check if we have imagemagick or other conversion tools
    if command -v convert &> /dev/null; then
      convert "$temp_png" "${OUTPUT_DIR}/${coin_symbol}.svg" 2>/dev/null && rm "$temp_png" && echo "✓ Converted ${coin_symbol}.png to SVG"
      return 0
    elif command -v rsvg-convert &> /dev/null; then
      rsvg-convert "$temp_png" -o "${OUTPUT_DIR}/${coin_symbol}.svg" 2>/dev/null && rm "$temp_png" && echo "✓ Converted ${coin_symbol}.png to SVG"
      return 0
    else
      echo "  PNG downloaded but no converter found. Keeping PNG: ${coin_symbol}_temp.png"
      return 1
    fi
  fi
  
  return 1
}

# Process coins
i=0
while [ $i -lt ${#COINS_NEEDED[@]} ]; do
  coin_symbol=${COINS_NEEDED[$i]}
  coin_name=${COINS_NEEDED[$((i+1))]}
  
  # Skip if we already have this coin
  if [ -f "${OUTPUT_DIR}/${coin_symbol}.svg" ]; then
    echo "⏭ Already have ${coin_symbol}.svg"
    i=$((i+2))
    continue
  fi
  
  # Try GitHub first
  if download_from_github "$coin_symbol" "$coin_name"; then
    i=$((i+2))
    continue
  fi
  
  # Try CoinGecko as fallback
  download_and_convert "$coin_symbol" "$coin_name"
  
  i=$((i+2))
done

echo ""
echo "Download complete!"
ls -lh "$OUTPUT_DIR"/*.svg 2>/dev/null | wc -l | xargs echo "Total SVG files:"
