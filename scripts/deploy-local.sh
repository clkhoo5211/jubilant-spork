#!/bin/bash
# NOFX Local Development Deployment Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "🚀 NOFX Local Deployment Script"
echo "================================"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Functions
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check prerequisites
echo "📋 Checking prerequisites..."
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Install it first: brew install go"
    exit 1
fi

if ! command -v node &> /dev/null; then
    print_error "Node.js is not installed. Install it first: brew install node"
    exit 1
fi

if ! command -v npm &> /dev/null; then
    print_error "npm is not installed. Install it first: brew install node"
    exit 1
fi

print_status "Prerequisites check passed"

# Stop existing processes
echo ""
echo "🛑 Stopping existing processes..."
pkill -f "./nofx" 2>/dev/null || true
pkill -f "vite|npm.*dev" 2>/dev/null || true
sleep 2

# Backend setup
echo ""
echo "🔧 Setting up backend..."
cd "$PROJECT_ROOT"

if [ ! -f "nofx" ]; then
    echo "Building backend..."
    go mod download
    go build -o nofx
    print_status "Backend built"
else
    print_status "Backend binary exists"
fi

# Check config
if [ ! -f "config.json" ]; then
    print_warning "config.json not found. Copying from example..."
    cp config.json.example config.json
    print_warning "Please edit config.json with your API keys!"
    read -p "Press Enter to continue after editing config.json..."
fi

# Frontend setup
echo ""
echo "🔧 Setting up frontend..."
cd "$PROJECT_ROOT/web"

if [ ! -d "node_modules" ]; then
    echo "Installing frontend dependencies..."
    npm install
    print_status "Frontend dependencies installed"
else
    print_status "Frontend dependencies already installed"
fi

# Start services
echo ""
echo "🚀 Starting services..."

# Start backend
cd "$PROJECT_ROOT"
echo "Starting backend..."
nohup ./nofx > nofx.log 2>&1 &
BACKEND_PID=$!
print_status "Backend started (PID: $BACKEND_PID)"

# Start frontend
cd "$PROJECT_ROOT/web"
echo "Starting frontend..."
nohup npm run dev > ../frontend.log 2>&1 &
FRONTEND_PID=$!
print_status "Frontend started (PID: $FRONTEND_PID)"

# Wait for services to start
echo ""
echo "⏳ Waiting for services to start..."
sleep 5

# Verify services
echo ""
echo "🔍 Verifying services..."

if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    print_status "Backend is running (http://localhost:8081/health)"
else
    print_error "Backend health check failed"
    echo "Check logs: tail -f $PROJECT_ROOT/nofx.log"
fi

if curl -s http://localhost:4000 > /dev/null 2>&1; then
    print_status "Frontend is running (http://localhost:4000)"
else
    print_error "Frontend check failed"
    echo "Check logs: tail -f $PROJECT_ROOT/frontend.log"
fi

echo ""
echo "✅ Deployment complete!"
echo ""
echo "📍 Access points:"
echo "   Frontend: http://localhost:4000"
echo "   Backend:  http://localhost:8081"
echo ""
echo "📊 View logs:"
echo "   Backend:  tail -f $PROJECT_ROOT/nofx.log"
echo "   Frontend: tail -f $PROJECT_ROOT/frontend.log"
echo ""
echo "🛑 To stop:"
echo "   pkill -f './nofx'"
echo "   pkill -f 'vite|npm.*dev'"

