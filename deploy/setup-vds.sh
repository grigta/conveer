#!/bin/bash
set -e

echo "========================================"
echo "  Conveer VDS Setup Script"
echo "========================================"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# Update system
echo ""
echo "Step 1: Updating system..."
apt-get update -qq
apt-get upgrade -y -qq
print_status "System updated"

# Install dependencies
echo ""
echo "Step 2: Installing dependencies..."
apt-get install -y -qq \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    git \
    htop \
    nano
print_status "Dependencies installed"

# Install Docker
echo ""
echo "Step 3: Installing Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    systemctl enable docker
    systemctl start docker
    print_status "Docker installed"
else
    print_warning "Docker already installed"
fi

# Install Docker Compose
echo ""
echo "Step 4: Installing Docker Compose..."
if ! command -v docker-compose &> /dev/null; then
    curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    print_status "Docker Compose installed"
else
    print_warning "Docker Compose already installed"
fi

# Create app directory
echo ""
echo "Step 5: Setting up application directory..."
APP_DIR="/opt/conveer"
mkdir -p $APP_DIR
cd $APP_DIR

# Clone repository (if not exists)
if [ ! -d "$APP_DIR/.git" ]; then
    echo "Cloning repository..."
    git clone https://github.com/grigta/conveer.git .
    print_status "Repository cloned"
else
    echo "Updating repository..."
    git pull origin main
    print_status "Repository updated"
fi

# Setup firewall
echo ""
echo "Step 6: Configuring firewall..."
if command -v ufw &> /dev/null; then
    ufw allow 22/tcp    # SSH
    ufw allow 80/tcp    # HTTP
    ufw allow 443/tcp   # HTTPS
    ufw allow 8080/tcp  # API Gateway
    ufw allow 3000/tcp  # Grafana
    ufw allow 9090/tcp  # Prometheus
    ufw --force enable
    print_status "Firewall configured"
else
    print_warning "UFW not found, skipping firewall setup"
fi

# Create swap if needed (for low RAM servers)
echo ""
echo "Step 7: Checking swap..."
if [ $(swapon --show | wc -l) -eq 0 ]; then
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    print_status "Swap created (2GB)"
else
    print_warning "Swap already exists"
fi

echo ""
echo "========================================"
echo -e "${GREEN}  Setup completed!${NC}"
echo "========================================"
echo ""
echo "Next steps:"
echo "  1. Create .env file: nano /opt/conveer/.env"
echo "  2. Start services: cd /opt/conveer && docker-compose up -d"
echo "  3. Check status: docker-compose ps"
echo ""

