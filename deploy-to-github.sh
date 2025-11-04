#!/bin/bash

# Deploy to GitHub Repository Script
# This script helps you push your code to GitHub and set up GitHub Pages

set -e

REPO_URL="https://github.com/clkhoo5211/jubilant-spork.git"
REPO_NAME="jubilant-spork"
CURRENT_DIR="/Users/khoo/Downloads/nofx-main"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}GitHub Deployment Setup${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Check if git is initialized
if [ ! -d ".git" ]; then
    echo -e "${YELLOW}Initializing git repository...${NC}"
    git init
    echo -e "${GREEN}✓ Git initialized${NC}\n"
else
    echo -e "${GREEN}✓ Git repository already initialized${NC}\n"
fi

# Check for .gitignore
if [ ! -f ".gitignore" ]; then
    echo -e "${YELLOW}Creating .gitignore...${NC}"
    cat > .gitignore << 'EOF'
# Binaries
nofx
*.exe
*.dll
*.so
*.dylib

# Test binary
*.test

# Output of the go coverage tool
*.out
coverage.out

# Dependencies
vendor/

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Environment
.env
config.json
config.db

# Logs
*.log
nofx.log
frontend.log

# Build outputs
dist/
build/
web/dist/
web/node_modules/

# Decision logs (optional - remove if you want to track these)
decision_logs/
EOF
    echo -e "${GREEN}✓ .gitignore created${NC}\n"
fi

# Check remote
if git remote get-url origin > /dev/null 2>&1; then
    CURRENT_REMOTE=$(git remote get-url origin)
    echo -e "${YELLOW}Current remote:${NC} $CURRENT_REMOTE"
    read -p "Change remote to $REPO_URL? [y/n]: " change_remote
    if [ "$change_remote" = "y" ]; then
        git remote set-url origin "$REPO_URL"
        echo -e "${GREEN}✓ Remote updated${NC}\n"
    fi
else
    echo -e "${YELLOW}Adding remote repository...${NC}"
    git remote add origin "$REPO_URL"
    echo -e "${GREEN}✓ Remote added${NC}\n"
fi

# Stage all files
echo -e "${YELLOW}Staging files...${NC}"
git add .
echo -e "${GREEN}✓ Files staged${NC}\n"

# Check if there are changes
if git diff --cached --quiet; then
    echo -e "${YELLOW}No changes to commit${NC}\n"
else
    read -p "Enter commit message [or press Enter for default]: " commit_msg
    if [ -z "$commit_msg" ]; then
        commit_msg="Deploy: $(date '+%Y-%m-%d %H:%M:%S')"
    fi
    
    echo -e "${YELLOW}Committing changes...${NC}"
    git commit -m "$commit_msg"
    echo -e "${GREEN}✓ Changes committed${NC}\n"
fi

# Push to GitHub
echo -e "${YELLOW}Pushing to GitHub...${NC}"
echo -e "${BLUE}Note:${NC} This will push to the main branch"
read -p "Continue? [y/n]: " confirm
if [ "$confirm" = "y" ]; then
    git branch -M main 2>/dev/null || true
    git push -u origin main || {
        echo -e "${RED}Push failed. You may need to authenticate.${NC}"
        echo -e "${YELLOW}Options:${NC}"
        echo "  1. Use GitHub CLI: gh auth login"
        echo "  2. Use SSH: git remote set-url origin git@github.com:clkhoo5211/jubilant-spork.git"
        echo "  3. Push manually: git push -u origin main"
        exit 1
    }
    echo -e "${GREEN}✓ Code pushed to GitHub${NC}\n"
else
    echo -e "${YELLOW}Push cancelled${NC}"
    exit 0
fi

# Next steps
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Deployment Setup Complete!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${YELLOW}Next Steps:${NC}\n"

echo "1. Enable GitHub Pages:"
echo -e "   ${BLUE}https://github.com/clkhoo5211/jubilant-spork/settings/pages${NC}"
echo "   - Source: GitHub Actions"
echo "   - Or: Deploy from branch: gh-pages or main\n"

echo "2. Set Backend API URL (if backend is on different host):"
echo -e "   ${BLUE}https://github.com/clkhoo5211/jubilant-spork/settings/secrets/actions${NC}"
echo "   - New secret: API_BASE_URL"
echo "   - Value: Your backend URL (e.g., https://your-backend.railway.app)\n"

echo "3. Deploy Backend to a hosting service:"
echo "   - Railway: https://railway.app"
echo "   - Render: https://render.com"
echo "   - Fly.io: https://fly.io"
echo "   - Or your own server\n"

echo "4. Monitor deployment:"
echo -e "   ${BLUE}https://github.com/clkhoo5211/jubilant-spork/actions${NC}\n"

echo -e "${GREEN}Frontend will be available at:${NC}"
echo -e "${BLUE}https://clkhoo5211.github.io/jubilant-spork/${NC}\n"

echo -e "${YELLOW}Note:${NC} GitHub Pages only hosts static files."
echo "Backend must be deployed separately (Railway, Render, etc.)\n"
