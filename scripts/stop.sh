#!/bin/bash
# NOFX Stop Script - Stops both local and Docker deployments

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
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

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Show help
show_help() {
    echo "NOFX Stop Script"
    echo "=================="
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help              Show this help message"
    echo "  -l, --local             Stop local processes only"
    echo "  -d, --docker            Stop Docker containers only"
    echo "  -a, --all               Stop both local and Docker (default)"
    echo "  --remove                Stop Docker and remove containers (docker compose down)"
    echo ""
    echo "Examples:"
    echo "  $0                      # Stop both local and Docker"
    echo "  $0 --local              # Stop local processes only"
    echo "  $0 --docker             # Stop Docker containers only"
    echo "  $0 --docker --remove    # Stop and remove Docker containers"
}

# Stop local processes
stop_local() {
    print_info "Stopping local processes..."
    
    local killed_something=false
    
    # Stop backend process
    if pgrep -f "./nofx" > /dev/null; then
        print_info "Found backend process, stopping..."
        pkill -f "./nofx" 2>/dev/null || true
        killed_something=true
        sleep 1
    fi
    
    # Stop frontend processes (vite, npm dev)
    if pgrep -f "vite|npm.*dev" > /dev/null; then
        print_info "Found frontend process, stopping..."
        pkill -f "vite|npm.*dev" 2>/dev/null || true
        killed_something=true
        sleep 1
    fi
    
    if [ "$killed_something" = true ]; then
        print_status "Local processes stopped"
    else
        print_warning "No local processes found running"
    fi
}

# Stop Docker containers
stop_docker() {
    local remove_containers=false
    if [ "$1" = "--remove" ]; then
        remove_containers=true
    fi
    
    print_info "Checking Docker deployment..."
    
    # Detect docker compose command
    if command -v docker compose &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        print_warning "Docker Compose not found, skipping Docker stop"
        return
    fi
    
    # Check if docker compose file exists
    if [ ! -f "docker-compose.yml" ] && [ ! -f "compose.yml" ]; then
        print_warning "No docker-compose.yml found, skipping Docker stop"
        return
    fi
    
    # Check if docker daemon is running
    if ! docker info > /dev/null 2>&1; then
        print_warning "Docker daemon is not running, skipping Docker stop"
        return
    fi
    
    # Check if any containers are running
    if ! $COMPOSE_CMD ps 2>/dev/null | grep -q "Up"; then
        print_warning "No running Docker containers found"
        return
    fi
    
    if [ "$remove_containers" = true ]; then
        print_info "Stopping and removing Docker containers..."
        $COMPOSE_CMD down
        print_status "Docker containers stopped and removed"
    else
        print_info "Stopping Docker containers..."
        $COMPOSE_CMD stop
        print_status "Docker containers stopped"
    fi
}

# Main execution
main() {
    local stop_local_only=false
    local stop_docker_only=false
    local remove_docker=false
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -l|--local)
                stop_local_only=true
                shift
                ;;
            -d|--docker)
                stop_docker_only=true
                shift
                ;;
            -a|--all)
                stop_local_only=false
                stop_docker_only=false
                shift
                ;;
            --remove)
                remove_docker=true
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                echo ""
                show_help
                exit 1
                ;;
        esac
    done
    
    echo "🛑 NOFX Stop Script"
    echo "==================="
    echo ""
    
    # Execute stop based on options
    if [ "$stop_local_only" = true ]; then
        stop_local
    elif [ "$stop_docker_only" = true ]; then
        if [ "$remove_docker" = true ]; then
            stop_docker "--remove"
        else
            stop_docker
        fi
    else
        # Stop both (default behavior)
        stop_local
        echo ""
        if [ "$remove_docker" = true ]; then
            stop_docker "--remove"
        else
            stop_docker
        fi
    fi
    
    echo ""
    print_status "Done!"
}

# Execute main function with all arguments
main "$@"

