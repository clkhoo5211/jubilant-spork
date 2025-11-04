#!/bin/bash
# NOFX Docker Production Deployment Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "🐳 NOFX Docker Deployment Script"
echo "================================="
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

# Detect server IP
detect_server_ip() {
    # Try to get the IP from various sources
    if command -v hostname &> /dev/null; then
        # Try hostname -I first (Linux)
        if IP=$(hostname -I 2>/dev/null); then
            SERVER_IP=$(echo $IP | awk '{print $1}')
        # Try hostname -i (macOS/Linux)
        elif IP=$(hostname -i 2>/dev/null); then
            SERVER_IP=$(echo $IP | awk '{print $1}')
        fi
    fi
    
    # If still not found, try ip command
    if [ -z "$SERVER_IP" ] && command -v ip &> /dev/null; then
        IP=$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K\S+')
        if [ ! -z "$IP" ]; then
            SERVER_IP=$IP
        fi
    fi
    
    # If still not found, default to localhost
    if [ -z "$SERVER_IP" ]; then
        SERVER_IP="localhost"
    fi
    
    echo "$SERVER_IP"
}

# Detect docker compose command
detect_compose_cmd() {
    if command -v docker compose &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        print_error "Docker Compose is not installed. Install it first."
        exit 1
    fi
    echo -e "${GREEN}ℹ${NC} Using Docker Compose command: $COMPOSE_CMD"
}

# Check prerequisites
echo "📋 Checking prerequisites..."
if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed. Install it first."
    exit 1
fi

detect_compose_cmd
print_status "Prerequisites check passed"

# Check docker-compose.yml exists
if [ ! -f "docker-compose.yml" ] && [ ! -f "compose.yml" ]; then
    print_error "docker-compose.yml or compose.yml not found!"
    print_error "Please make sure you're running this script from the project root."
    exit 1
fi

# Check config
if [ ! -f "config.json" ]; then
    print_warning "config.json not found. Copying from example..."
    cp config.json.example config.json
    print_warning "Please edit config.json with your API keys!"
    read -p "Press Enter to continue after editing config.json..."
fi

# Parse command line arguments
ACTION="${1:-start}"
BUILD="${2:-no}"

case "$ACTION" in
    start)
        echo ""
        echo "🚀 Starting services..."
        
        if [ "$BUILD" = "build" ] || [ "$BUILD" = "--build" ]; then
            $COMPOSE_CMD up -d --build
            print_status "Services built and started"
        else
            $COMPOSE_CMD up -d
            print_status "Services started"
        fi
        
        echo ""
        echo "⏳ Waiting for services to start..."
        sleep 5
        
        # Verify services
        echo ""
        echo "🔍 Verifying services..."
        
        if $COMPOSE_CMD ps | grep -q "Up"; then
            print_status "Containers are running"
            $COMPOSE_CMD ps
        else
            print_error "Some containers failed to start"
            $COMPOSE_CMD ps
            exit 1
        fi
        
        if curl -s http://localhost:${NOFX_BACKEND_PORT:-8888}/health > /dev/null 2>&1; then
            print_status "Backend health check passed (http://localhost:${NOFX_BACKEND_PORT:-8888}/health)"
        else
            print_warning "Backend health check failed (may still be starting)"
        fi
        
        if curl -s http://localhost:${NOFX_FRONTEND_PORT:-3333} > /dev/null 2>&1; then
            print_status "Frontend check passed (http://localhost:${NOFX_FRONTEND_PORT:-3333})"
        else
            print_warning "Frontend check failed (may still be starting)"
        fi
        
        echo ""
        echo "✅ Deployment complete!"
        echo ""
        
        # Detect server IP and domain
        SERVER_IP=$(detect_server_ip)
        
        # Check if domain name is set via environment variable
        if [ ! -z "$NOFX_DOMAIN" ]; then
            SERVER_HOST="$NOFX_DOMAIN"
        elif [ "$SERVER_IP" != "localhost" ]; then
            SERVER_HOST="$SERVER_IP"
        else
            SERVER_HOST="localhost"
        fi
        
        echo "📍 Access points:"
        echo "   Frontend: http://$SERVER_HOST:${NOFX_FRONTEND_PORT:-3333}"
        echo "   Backend:  http://$SERVER_HOST:${NOFX_BACKEND_PORT:-8888}"
        if [ "$SERVER_HOST" != "localhost" ] && [ "$SERVER_HOST" != "$SERVER_IP" ]; then
            echo "   (or use http://localhost:${NOFX_FRONTEND_PORT:-3333} from this server)"
        fi
        echo ""
        echo "📊 View logs:"
        echo "   $COMPOSE_CMD logs -f"
        echo ""
        echo "🛑 To stop:"
        echo "   $COMPOSE_CMD stop"
        ;;
        
    stop)
        echo ""
        echo "🛑 Stopping services..."
        $COMPOSE_CMD stop
        print_status "Services stopped"
        ;;
        
    restart)
        echo ""
        echo "🔄 Restarting services..."
        $COMPOSE_CMD restart
        print_status "Services restarted"
        ;;
        
    down)
        echo ""
        echo "🗑️  Stopping and removing containers..."
        $COMPOSE_CMD down
        print_status "Containers removed"
        ;;
        
    logs)
        echo ""
        echo "📊 Showing logs..."
        $COMPOSE_CMD logs -f "${2:-}"
        ;;
        
    status)
        echo ""
        echo "📊 Service status:"
        $COMPOSE_CMD ps
        echo ""
        echo "📈 Resource usage:"
        docker stats --no-stream
        ;;
        
    build)
        echo ""
        echo "🔨 Building images..."
        $COMPOSE_CMD build --no-cache
        print_status "Images built"
        ;;
        
    update)
        echo ""
        echo "🔄 Updating services..."
        git pull
        $COMPOSE_CMD down
        $COMPOSE_CMD up -d --build
        print_status "Services updated"
        ;;
        
    *)
        echo "Usage: $0 {start|stop|restart|down|logs|status|build|update} [--build]"
        echo ""
        echo "Commands:"
        echo "  start     - Start services (add --build to rebuild)"
        echo "  stop      - Stop services"
        echo "  restart   - Restart services"
        echo "  down      - Stop and remove containers"
        echo "  logs      - Show logs (optionally for specific service)"
        echo "  status    - Show service status and resource usage"
        echo "  build     - Build images without cache"
        echo "  update    - Pull latest code and rebuild"
        echo ""
        echo "Examples:"
        echo "  $0 start          # Start without rebuild"
        echo "  $0 start --build  # Start with rebuild"
        echo "  $0 logs nofx     # Show backend logs only"
        exit 1
        ;;
esac

