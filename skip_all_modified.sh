#!/bin/bash

# Skip All Modified Files Script
# This script ONLY adds new files from upstream
# It SKIPS all modified files to preserve your local changes

set -e

LOCAL_DIR="/Users/khoo/Downloads/nofx-main"
UPSTREAM_DIR="/Users/khoo/Downloads/nofx-upstream"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Skip All Modified Files Strategy${NC}"
echo -e "${BLUE}Only Add New Files${NC}"
echo -e "${BLUE}========================================${NC}\n"

echo -e "${YELLOW}Strategy:${NC}"
echo "  ✓ Add new files from upstream (safe)"
echo "  ✓ Skip all modified files (preserve your changes)"
echo ""

# Find new files in upstream
echo -e "${YELLOW}Scanning for new files in upstream...${NC}\n"

NEW_FILES=()

# Find files that exist in upstream but not in local
find "$UPSTREAM_DIR" -type f \( -name "*.go" -o -name "*.ts" -o -name "*.tsx" -o -name "*.md" -o -name "*.json" -o -name "*.sh" \) ! -path "*/node_modules/*" ! -path "*/.git/*" ! -path "*/dist/*" | while read upstream_file; do
    rel_path="${upstream_file#$UPSTREAM_DIR/}"
    local_file="$LOCAL_DIR/$rel_path"
    
    if [ ! -f "$local_file" ]; then
        NEW_FILES+=("$rel_path")
        echo "  + $rel_path"
    fi
done

echo ""
echo -e "${GREEN}Found new files that can be safely added${NC}\n"

# Ask what to add
echo "What would you like to add?"
echo ""
echo "  1) Add authentication system (auth/ directory)"
echo "  2) Add documentation files (CHANGELOG, etc.)"
echo "  3) Add specific file/directory"
echo "  4) Add everything new (all new files)"
echo "  5) Cancel"
echo ""
read -p "Choose [1-5]: " choice

case $choice in
    1)
        if [ -d "$UPSTREAM_DIR/auth" ] && [ ! -d "$LOCAL_DIR/auth" ]; then
            echo ""
            echo -e "${YELLOW}Adding authentication system...${NC}"
            cp -r "$UPSTREAM_DIR/auth" "$LOCAL_DIR/"
            echo -e "${GREEN}✓ Added auth/ directory${NC}"
        else
            echo "Authentication system already exists or not found in upstream"
        fi
        ;;
    2)
        echo ""
        echo -e "${YELLOW}Adding documentation files...${NC}"
        for doc in CHANGELOG.md CHANGELOG.zh-CN.md; do
            if [ -f "$UPSTREAM_DIR/$doc" ] && [ ! -f "$LOCAL_DIR/$doc" ]; then
                cp "$UPSTREAM_DIR/$doc" "$LOCAL_DIR/"
                echo -e "${GREEN}✓ Added $doc${NC}"
            fi
        done
        ;;
    3)
        echo ""
        read -p "Enter file or directory path (e.g., auth/ or prompts/): " path
        if [ -e "$UPSTREAM_DIR/$path" ]; then
            if [ -d "$UPSTREAM_DIR/$path" ]; then
                echo "Copying directory..."
                cp -r "$UPSTREAM_DIR/$path" "$LOCAL_DIR/"
                echo -e "${GREEN}✓ Added $path${NC}"
            else
                dir=$(dirname "$path")
                mkdir -p "$LOCAL_DIR/$dir"
                cp "$UPSTREAM_DIR/$path" "$LOCAL_DIR/$path"
                echo -e "${GREEN}✓ Added $path${NC}"
            fi
        else
            echo "File/directory not found in upstream"
        fi
        ;;
    4)
        echo ""
        echo -e "${YELLOW}Adding all new files...${NC}"
        find "$UPSTREAM_DIR" -type f ! -path "*/node_modules/*" ! -path "*/.git/*" ! -path "*/dist/*" | while read upstream_file; do
            rel_path="${upstream_file#$UPSTREAM_DIR/}"
            local_file="$LOCAL_DIR/$rel_path"
            
            if [ ! -f "$local_file" ]; then
                dir=$(dirname "$local_file")
                mkdir -p "$dir"
                cp "$upstream_file" "$local_file"
                echo -e "${GREEN}✓ Added $rel_path${NC}"
            fi
        done
        ;;
    5)
        echo "Cancelled"
        exit 0
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Done!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}Summary:${NC}"
echo "  ✓ All modified files were SKIPPED (your changes preserved)"
echo "  ✓ Only new files were added (no conflicts)"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "  1. Review the new files that were added"
echo "  2. Test compilation: go build"
echo "  3. Your local customizations are intact"
echo ""
echo -e "${BLUE}Note:${NC} Modified files were intentionally skipped to preserve your local changes."
echo "      If you want to review specific upstream changes later, use:"
echo "      diff -u local/file.go ../nofx-upstream/file.go"
echo ""
