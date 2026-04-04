#!/bin/bash
set -e

APP_NAME="rdxclaw"
INSTALL_DIR="$HOME"
BINARY_PATH="$INSTALL_DIR/$APP_NAME"
LOG_FILE="$INSTALL_DIR/$APP_NAME.log"
SERVICE_NAME="$APP_NAME.service"
SERVICE_PATH="/etc/systemd/system/$SERVICE_NAME"
PORT="${RDXCLAW_PORT:-8080}"

echo "🚀 Starting Deployment for $APP_NAME"

# ─── 1. Prepare Binary ───────────────────────────────────────────────────────
if [ -f "/tmp/build/$APP_NAME-linux-amd64" ]; then
    echo "📦 Moving binary from /tmp/build..."
    mv "/tmp/build/$APP_NAME-linux-amd64" "$BINARY_PATH"
elif [ -f "/tmp/$APP_NAME-linux-amd64" ]; then
    echo "📦 Moving binary from /tmp..."
    mv "/tmp/$APP_NAME-linux-amd64" "$BINARY_PATH"
elif [ -f "/tmp/$APP_NAME" ]; then
    echo "📦 Moving binary from /tmp..."
    mv "/tmp/$APP_NAME" "$BINARY_PATH"
else
    echo "❌ No binary found in /tmp or /tmp/build"
    exit 1
fi
chmod +x "$BINARY_PATH"
echo "✅ Binary installed to $BINARY_PATH"

# ─── 2. Config Check ─────────────────────────────────────────────────────────
CONFIG_DIR="$HOME/.rdxclaw"
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.json" ]; then
    echo "⚠️  No config.json found in $CONFIG_DIR"
    echo "Initializing default config..."
    "$BINARY_PATH" onboard --non-interactive <<< "3" || true
fi

# ─── 3. Service Management ───────────────────────────────────────────────────
use_systemd() {
    command -v systemctl >/dev/null 2>&1 || return 1
    systemctl --version >/dev/null 2>&1 || return 1

    # Can we write the service file? (root or sudo without password)
    if [ "$EUID" -eq 0 ]; then
        return 0
    fi

    # Test if passwordless sudo is available for systemctl
    if sudo -n systemctl status >/dev/null 2>&1; then
        return 0
    fi

    return 1
}

write_service_file() {
    local content="[Unit]
Description=RDxClaw Agent Service
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$HOME
EnvironmentFile=-$HOME/.rdxclaw/env
ExecStart=$BINARY_PATH server --port $PORT
Restart=always
RestartSec=5
StandardOutput=append:$LOG_FILE
StandardError=append:$LOG_FILE

[Install]
WantedBy=multi-user.target"

    if [ "$EUID" -eq 0 ]; then
        echo "$content" > "$SERVICE_PATH"
    else
        echo "$content" | sudo tee "$SERVICE_PATH" > /dev/null
    fi
}

run_systemctl() {
    if [ "$EUID" -eq 0 ]; then
        systemctl "$@"
    else
        sudo systemctl "$@"
    fi
}

if use_systemd; then
    echo "⚙️  Setting up systemd service..."
    write_service_file
    run_systemctl daemon-reload
    run_systemctl enable "$SERVICE_NAME"
    run_systemctl restart "$SERVICE_NAME"
    echo "✅ systemd service $SERVICE_NAME restarted"
    USING_SYSTEMD=true
else
    echo "⚠️  Systemd unavailable or no sudo access — falling back to nohup..."
    pkill -f "$APP_NAME server" 2>/dev/null || true
    sleep 1
    nohup "$BINARY_PATH" server --port "$PORT" > "$LOG_FILE" 2>&1 &
    NOHUP_PID=$!
    echo "✅ Process started in background (PID: $NOHUP_PID)"
    USING_SYSTEMD=false
fi

# ─── 4. Status Check ─────────────────────────────────────────────────────────
echo "📊 Status Check:"
sleep 2

if [ "$USING_SYSTEMD" = true ]; then
    run_systemctl status "$SERVICE_NAME" --no-pager || true
else
    ps aux | grep "$APP_NAME server" | grep -v grep || echo "⚠️  Process not found in ps"
    echo "--- Last 20 lines of log ---"
    tail -n 20 "$LOG_FILE" 2>/dev/null || true
fi

# ─── 5. Health Check ─────────────────────────────────────────────────────────
echo "🔍 Performing local health check on port $PORT..."
MAX_RETRIES=6
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s -f "http://localhost:$PORT/health" > /dev/null; then
        echo "✅ Health check passed!"
        exit 0
    fi
    RETRY_COUNT=$((RETRY_COUNT+1))
    echo "⏳ Waiting for service to initialize... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 5
done

echo "❌ Health check failed after $MAX_RETRIES attempts"
if [ "$USING_SYSTEMD" = true ]; then
    echo "--- Last 30 lines of journal ---"
    sudo journalctl -u "$SERVICE_NAME" --no-pager -n 30 || true
else
    echo "--- Last 30 lines of log ---"
    tail -n 30 "$LOG_FILE" 2>/dev/null || true
fi
exit 1