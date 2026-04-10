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
    StandardOutput=journal
    StandardError=journal

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

    # Graceful upgrade: send SIGUSR2 to the running process instead of restarting
    PID=$(run_systemctl show -p MainPID --value "$SERVICE_NAME" 2>/dev/null || echo "0")
    if [ -n "$PID" ] && [ "$PID" != "0" ]; then
        echo "🔄 Sending graceful upgrade signal (SIGUSR2) to PID $PID..."
        kill -USR2 "$PID" 2>/dev/null || true

        # Wait for handoff to complete (the new process inherits the sockets)
        echo "⏳ Waiting for binary handoff..."
        MAX_HANDOFF_WAIT=15
        HANDOFF_COUNT=0
        while [ $HANDOFF_COUNT -lt $MAX_HANDOFF_WAIT ]; do
            NEW_PID=$(run_systemctl show -p MainPID --value "$SERVICE_NAME" 2>/dev/null || echo "0")
            if [ "$NEW_PID" != "$PID" ] && [ "$NEW_PID" != "0" ]; then
                echo "✅ Binary handoff complete (old PID: $PID → new PID: $NEW_PID)"
                break
            fi
            HANDOFF_COUNT=$((HANDOFF_COUNT+1))
            sleep 1
        done

        if [ $HANDOFF_COUNT -ge $MAX_HANDOFF_WAIT ]; then
            echo "⚠️  Handoff timeout — falling back to restart"
            run_systemctl restart "$SERVICE_NAME"
        fi
    else
        echo "⚠️  No running process found — starting fresh"
        run_systemctl start "$SERVICE_NAME"
    fi
    echo "✅ systemd service $SERVICE_NAME updated"
    USING_SYSTEMD=true
else
    echo "⚠️  Systemd unavailable or no sudo access — falling back to nohup..."
    # For nohup mode, send SIGUSR2 if a process is running
    OLD_PID=$(pgrep -f "$APP_NAME server" 2>/dev/null || echo "")
    if [ -n "$OLD_PID" ]; then
        echo "🔄 Sending graceful upgrade signal (SIGUSR2) to PID $OLD_PID..."
        kill -USR2 "$OLD_PID" 2>/dev/null || true
        sleep 5
        # Check if old process exited
        if kill -0 "$OLD_PID" 2>/dev/null; then
            echo "⚠️  Old process still running — force killing"
            kill -9 "$OLD_PID" 2>/dev/null || true
            sleep 1
        fi
    fi
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