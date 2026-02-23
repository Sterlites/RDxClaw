#!/bin/bash
set -e

APP_NAME="rdxclaw"
INSTALL_DIR="$HOME"
BINARY_PATH="$INSTALL_DIR/$APP_NAME"
LOG_FILE="$INSTALL_DIR/$APP_NAME.log"
SERVICE_NAME="$APP_NAME.service"

echo "🚀 Starting Deployment for $APP_NAME"

# 1. Prepare Binary
if [ -f "/tmp/build/$APP_NAME-linux-amd64" ]; then
    echo "📦 Moving binary from /tmp/build..."
    mv "/tmp/build/$APP_NAME-linux-amd64" "$BINARY_PATH"
elif [ -f "/tmp/$APP_NAME-linux-amd64" ]; then
    echo "📦 Moving binary from /tmp..."
    mv "/tmp/$APP_NAME-linux-amd64" "$BINARY_PATH"
elif [ -f "/tmp/$APP_NAME" ]; then
    echo "📦 Moving binary from /tmp..."
    mv "/tmp/$APP_NAME" "$BINARY_PATH"
fi

chmod +x "$BINARY_PATH"

# 2. Config Check
CONFIG_DIR="$HOME/.rdxclaw"
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.json" ]; then
    echo "⚠️ Warning: No config.json found in $CONFIG_DIR"
    echo "Initializing default config..."
    "$BINARY_PATH" onboard --non-interactive <<EOF
3
EOF
fi

# 3. Service Management
if command -v systemctl >/dev/null 2>&1 && [ "$EUID" -eq 0 ]; then
    echo "⚙️ Systemd detected and running as root, setting up service..."
    # This part usually requires root, but if the user is 'deploy' they might have sudo
    # Let's assume for now we might need to use sudo or fallback to nohup
    
    # Template replacement (simplified for shell)
    SERVICE_PATH="/etc/systemd/system/$SERVICE_NAME"
    cat <<EOF | sudo tee "$SERVICE_PATH" > /dev/null
[Unit]
Description=RDxClaw Agent Service
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$HOME
ExecStart=$BINARY_PATH server --port 8080
Restart=always
RestartSec=5
StandardOutput=append:$LOG_FILE
StandardError=append:$LOG_FILE

[Install]
WantedBy=multi-user.target
EOF
    sudo systemctl daemon-reload
    sudo systemctl enable "$SERVICE_NAME"
    sudo systemctl restart "$SERVICE_NAME"
    echo "✅ Service $SERVICE_NAME restarted"
else
    echo "⚠️ Systemd not used or no permissions, falling back to nohup..."
    pkill -f "$APP_NAME server" || true
    nohup "$BINARY_PATH" server --port 8080 > "$LOG_FILE" 2>&1 &
    echo "✅ Process started in background (nohup)"
fi

echo "📊 Status Check:"
sleep 2
ps aux | grep "$APP_NAME server" | grep -v grep
tail -n 20 "$LOG_FILE"

# 4. Local Health Check
echo "🔍 Performing local health check..."
MAX_RETRIES=6
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s -f http://localhost:8080/health > /dev/null; then
        echo "✅ Local health check passed!"
        exit 0
    fi
    echo "Wait for service to initialize... ($((RETRY_COUNT+1))/$MAX_RETRIES)"
    RETRY_COUNT=$((RETRY_COUNT+1))
    sleep 5
done

echo "❌ Local health check failed after $MAX_RETRIES attempts"
exit 1
