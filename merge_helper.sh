#!/bin/bash

# Merge Helper Script for Upstream Changes
# This script helps you safely merge upstream changes while preserving local customizations

set -e

LOCAL_DIR="/Users/khoo/Downloads/nofx-main"
UPSTREAM_DIR="/Users/khoo/Downloads/nofx-upstream"
BACKUP_DIR="/Users/khoo/Downloads/nofx-main-backup-$(date +%Y%m%d-%H%M%S)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Upstream Merge Helper${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Function to check if file exists in both
check_file() {
    local file=$1
    if [ -f "$LOCAL_DIR/$file" ] && [ -f "$UPSTREAM_DIR/$file" ]; then
        return 0
    fi
    return 1
}

# Function to backup file
backup_file() {
    local file=$1
    local dir=$(dirname "$BACKUP_DIR/$file")
    mkdir -p "$dir"
    cp "$LOCAL_DIR/$file" "$BACKUP_DIR/$file" 2>/dev/null || true
}

# Step 1: Create backup
echo -e "${YELLOW}Step 1: Creating backup...${NC}"
mkdir -p "$BACKUP_DIR"
echo "Backup location: $BACKUP_DIR"
echo ""

# Step 2: Show what will be merged
echo -e "${YELLOW}Step 2: Files that will be compared:${NC}"
echo ""

# High priority files
HIGH_PRIORITY=(
    "main.go"
    "api/server.go"
    "config/config.go"
)

echo -e "${RED}High Priority Files (Manual Review Required):${NC}"
for file in "${HIGH_PRIORITY[@]}"; do
    if check_file "$file"; then
        if ! diff -q "$LOCAL_DIR/$file" "$UPSTREAM_DIR/$file" > /dev/null 2>&1; then
            echo -e "  ${YELLOW}⚠️  $file${NC} - Modified in upstream"
            backup_file "$file"
        else
            echo -e "  ${GREEN}✓  $file${NC} - No changes"
        fi
    else
        if [ -f "$UPSTREAM_DIR/$file" ]; then
            echo -e "  ${BLUE}+  $file${NC} - New in upstream"
        fi
    fi
done
echo ""

# Step 3: Show new files in upstream
echo -e "${YELLOW}Step 3: New files in upstream (can be safely added):${NC}"
NEW_FILES=(
    "auth/auth.go"
    "CHANGELOG.md"
    "CHANGELOG.zh-CN.md"
)

for file in "${NEW_FILES[@]}"; do
    if [ -f "$UPSTREAM_DIR/$file" ] && [ ! -f "$LOCAL_DIR/$file" ]; then
        echo -e "  ${GREEN}+  $file${NC} - Can be added"
    fi
done
echo ""

# Step 4: Interactive merge options
echo -e "${YELLOW}Step 4: Merge Options${NC}"
echo ""
echo "Choose an option:"
echo "  1) View diff for a specific file"
echo "  2) Merge a specific file (creates backup first)"
echo "  3) Add new upstream files"
echo "  4) Show summary of all changes"
echo "  5) Exit"
echo ""
read -p "Enter choice [1-5]: " choice

case $choice in
    1)
        echo ""
        read -p "Enter file path (e.g., api/server.go): " filepath
        if check_file "$filepath"; then
            echo ""
            echo -e "${BLUE}=== Differences in $filepath ===${NC}"
            diff -u "$LOCAL_DIR/$filepath" "$UPSTREAM_DIR/$filepath" | head -100 || true
            echo ""
            echo -e "${YELLOW}Note: Use a merge tool (meld, kdiff3, vscode) for full comparison${NC}"
        else
            echo "File not found in both directories"
        fi
        ;;
    2)
        echo ""
        read -p "Enter file path to merge (e.g., api/server.go): " filepath
        if check_file "$filepath"; then
            backup_file "$filepath"
            echo "Backup created. Now showing diff..."
            diff -u "$LOCAL_DIR/$filepath" "$UPSTREAM_DIR/$filepath" | head -50 || true
            echo ""
            read -p "Do you want to copy upstream version? (yes/no): " confirm
            if [ "$confirm" = "yes" ]; then
                cp "$UPSTREAM_DIR/$filepath" "$LOCAL_DIR/$filepath"
                echo -e "${GREEN}File merged. Original backed up to $BACKUP_DIR/$filepath${NC}"
            else
                echo "Merge cancelled. Use manual merge instead."
            fi
        else
            echo "File not found in both directories"
        fi
        ;;
    3)
        echo ""
        echo "Adding new upstream files..."
        if [ ! -d "$LOCAL_DIR/auth" ] && [ -d "$UPSTREAM_DIR/auth" ]; then
            read -p "Add authentication system? (yes/no): " confirm
            if [ "$confirm" = "yes" ]; then
                cp -r "$UPSTREAM_DIR/auth" "$LOCAL_DIR/"
                echo -e "${GREEN}Authentication system added${NC}"
            fi
        fi
        echo ""
        read -p "Add documentation files? (yes/no): " confirm
        if [ "$confirm" = "yes" ]; then
            for file in CHANGELOG.md CHANGELOG.zh-CN.md; do
                if [ -f "$UPSTREAM_DIR/$file" ] && [ ! -f "$LOCAL_DIR/$file" ]; then
                    cp "$UPSTREAM_DIR/$file" "$LOCAL_DIR/"
                    echo -e "${GREEN}Added $file${NC}"
                fi
            done
        fi
        ;;
    4)
        echo ""
        echo -e "${BLUE}=== Summary ===${NC}"
        echo "Modified files:"
        find "$UPSTREAM_DIR" -type f -name "*.go" | while read upstream_file; do
            rel_path="${upstream_file#$UPSTREAM_DIR/}"
            local_file="$LOCAL_DIR/$rel_path"
            if [ -f "$local_file" ]; then
                if ! diff -q "$local_file" "$upstream_file" > /dev/null 2>&1; then
                    echo "  M $rel_path"
                fi
            fi
        done
        echo ""
        echo "New files:"
        find "$UPSTREAM_DIR" -type f -name "*.go" | while read upstream_file; do
            rel_path="${upstream_file#$UPSTREAM_DIR/}"
            local_file="$LOCAL_DIR/$rel_path"
            if [ ! -f "$local_file" ]; then
                echo "  + $rel_path"
            fi
        done
        ;;
    5)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo "Invalid choice"
        ;;
esac

echo ""
echo -e "${GREEN}Done! Backup saved to: $BACKUP_DIR${NC}"
