#!/bin/bash

# Selective Merge Script - Preserves your local changes
# This script allows you to choose which files to merge and which to skip

set -e

LOCAL_DIR="/Users/khoo/Downloads/nofx-main"
UPSTREAM_DIR="/Users/khoo/Downloads/nofx-upstream"
BACKUP_DIR="/Users/khoo/Downloads/nofx-main-backup-$(date +%Y%m%d-%H%M%S)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Selective Merge Tool${NC}"
echo -e "${BLUE}Preserve Local Changes${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Create backup
echo -e "${YELLOW}Creating backup...${NC}"
mkdir -p "$BACKUP_DIR"
echo "Backup: $BACKUP_DIR"
echo ""

# Files to ALWAYS skip (your customizations)
ALWAYS_SKIP=(
    "trader/gateio_trader.go"
    "market/gateio.go"
    "config.json"  # Your config should be preserved manually
    "cmd/test_alpaca_provider/"
    "cmd/test_alpaca_crypto/"
    "ALPACA_INTEGRATION.md"
)

# Function to check if file should always be skipped
should_always_skip() {
    local file=$1
    for skip_pattern in "${ALWAYS_SKIP[@]}"; do
        if [[ "$file" == *"$skip_pattern"* ]]; then
            return 0
        fi
    done
    return 1
}

# Function to backup file
backup_file() {
    local file=$1
    local dir=$(dirname "$BACKUP_DIR/$file")
    mkdir -p "$dir"
    cp "$LOCAL_DIR/$file" "$BACKUP_DIR/$file" 2>/dev/null || true
}

echo -e "${GREEN}Strategy: Skip files with local customizations, merge others selectively${NC}\n"

# Find modified files
echo -e "${YELLOW}Scanning for modified files...${NC}\n"

MODIFIED_FILES=()

# Check Go files
find "$UPSTREAM_DIR" -name "*.go" -type f | while read upstream_file; do
    rel_path="${upstream_file#$UPSTREAM_DIR/}"
    local_file="$LOCAL_DIR/$rel_path"
    
    if [ -f "$local_file" ]; then
        # Check if files differ
        if ! diff -q "$local_file" "$upstream_file" > /dev/null 2>&1; then
            MODIFIED_FILES+=("$rel_path")
        fi
    fi
done

# Process each modified file
for rel_path in "${MODIFIED_FILES[@]}"; do
    # Skip files that should always be preserved
    if should_always_skip "$rel_path"; then
        echo -e "${GREEN}✓ SKIP:${NC} $rel_path (local customization preserved)"
        continue
    fi
    
    local_file="$LOCAL_DIR/$rel_path"
    upstream_file="$UPSTREAM_DIR/$rel_path"
    
    echo -e "\n${YELLOW}File:${NC} $rel_path"
    echo -e "${BLUE}Upstream has changes${NC}"
    echo ""
    echo "Options:"
    echo "  1) Skip - Keep your local version (recommended for customizations)"
    echo "  2) View diff - See what changed"
    echo "  3) Merge manually - I'll open merge tool"
    echo "  4) Skip all remaining - Skip all remaining files"
    echo ""
    read -p "Choose [1-4]: " choice
    
    case $choice in
        1)
            echo -e "${GREEN}✓ SKIPPED:${NC} Keeping your local version"
            ;;
        2)
            echo ""
            echo -e "${BLUE}=== Diff ===${NC}"
            diff -u "$local_file" "$upstream_file" | head -50 || true
            echo ""
            read -p "Press Enter to continue..."
            # Ask again after viewing
            read -p "Skip this file? [y/n]: " skip
            if [ "$skip" = "y" ]; then
                echo -e "${GREEN}✓ SKIPPED:${NC} Keeping your local version"
            else
                backup_file "$rel_path"
                echo "Opening merge tool..."
                if command -v meld > /dev/null; then
                    meld "$local_file" "$upstream_file" &
                elif command -v code > /dev/null; then
                    code --diff "$local_file" "$upstream_file"
                else
                    echo "No merge tool found. Manual merge required."
                fi
            fi
            ;;
        3)
            backup_file "$rel_path"
            if command -v meld > /dev/null; then
                meld "$local_file" "$upstream_file" &
            elif command -v code > /dev/null; then
                code --diff "$local_file" "$upstream_file"
            else
                echo "No merge tool found. Please merge manually."
            fi
            ;;
        4)
            echo -e "${YELLOW}Skipping all remaining files...${NC}"
            break
            ;;
        *)
            echo -e "${RED}Invalid choice, skipping...${NC}"
            ;;
    esac
done

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Selective merge complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Summary:"
echo "- Files with local customizations: SKIPPED (preserved)"
echo "- Other files: Processed based on your choices"
echo "- Backup saved to: $BACKUP_DIR"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Review changes"
echo "2. Test compilation: go build"
echo "3. Test your Gate.io trader"
echo "4. If issues, restore from backup"
echo ""
