# Gateway Service Management

The nekobot gateway can run as a system service, allowing it to start automatically on system boot and be managed by the system service manager.

## Supported Platforms

- **Linux**: systemd
- **macOS**: launchd
- **Windows**: Windows Service Manager

## Usage

### Running in Foreground (Default)

```bash
# Start gateway in foreground mode
nekobot gateway

# Or explicitly use the run command
nekobot gateway run
```

This is useful for:
- Development and testing
- Manual operation
- Debugging

### Installing as System Service

```bash
# Install the service (requires sudo/admin privileges)
sudo nekobot gateway install
```

This will:
- Register the gateway with your system service manager
- Configure it to start automatically on boot
- Create necessary service configuration files

### Service Management

```bash
# Start the service
sudo nekobot gateway start

# Stop the service
sudo nekobot gateway stop

# Restart the service
sudo nekobot gateway restart

# Check service status
nekobot gateway status

# Uninstall the service
sudo nekobot gateway uninstall
```

## Platform-Specific Commands

### Linux (systemd)

After installation, you can also use systemctl:

```bash
# Check status
sudo systemctl status nekobot-gateway

# View logs
sudo journalctl -u nekobot-gateway -f

# Start on boot
sudo systemctl enable nekobot-gateway

# Disable start on boot
sudo systemctl disable nekobot-gateway
```

### macOS (launchd)

After installation, the service is managed by launchd:

```bash
# Check status
sudo launchctl list | grep nekobot

# View logs
tail -f /var/log/nekobot-gateway.log
```

### Windows

After installation, use the Services management console:

```powershell
# Or use PowerShell
Get-Service nekobot-gateway
Start-Service nekobot-gateway
Stop-Service nekobot-gateway
```

## Configuration

The service uses the same configuration file as the foreground mode:
- Linux/macOS: `~/.nekobot/config.json`
- Windows: `%USERPROFILE%\.nekobot\config.json`

Make sure your configuration file is properly set up before installing the service.

## Logging

When running as a service, logs are written to:
- Linux: System journal (view with `journalctl`)
- macOS: `/var/log/nekobot-gateway.log`
- Windows: Windows Event Log

You can also configure file logging in `config.json`:

```json
{
  "logging": {
    "level": "info",
    "output": "both",
    "file_path": "/var/log/nekobot/gateway.log"
  }
}
```

## Troubleshooting

### Service fails to start

1. Check if the service is installed:
   ```bash
   nekobot gateway status
   ```

2. Verify configuration:
   ```bash
   nekobot agent -m "test"  # Test if basic config works
   ```

3. Check logs:
   ```bash
   # Linux
   sudo journalctl -u nekobot-gateway -n 50

   # macOS
   tail -50 /var/log/nekobot-gateway.log
   ```

### Permission issues

Make sure you run install/start/stop commands with appropriate privileges:
- Linux/macOS: Use `sudo`
- Windows: Run as Administrator

### Service not starting on boot

```bash
# Linux
sudo systemctl enable nekobot-gateway

# Windows
sc config nekobot-gateway start=auto
```

## Security Considerations

When running as a service:

1. **Configuration Security**: Protect your config file as it contains API keys
   ```bash
   chmod 600 ~/.nekobot/config.json
   ```

2. **Service User**: Consider running the service as a dedicated user
   ```bash
   # Linux: Edit the service file
   sudo systemctl edit nekobot-gateway
   # Add: User=nekobot
   ```

3. **API Key Management**: Use environment variables for sensitive data
   ```bash
   export NEKOBOT_PROVIDERS_ANTHROPIC_API_KEY="your-key"
   ```

## Examples

### Basic Setup

```bash
# 1. Configure nekobot
nekobot config init

# 2. Test in foreground
nekobot gateway

# 3. Install as service
sudo nekobot gateway install

# 4. Start the service
sudo nekobot gateway start

# 5. Check status
nekobot gateway status
```

### Update and Restart

```bash
# 1. Stop the service
sudo nekobot gateway stop

# 2. Update configuration
vim ~/.nekobot/config.json

# 3. Start the service
sudo nekobot gateway start
```

### Complete Uninstall

```bash
# 1. Stop the service
sudo nekobot gateway stop

# 2. Uninstall the service
sudo nekobot gateway uninstall

# 3. Remove configuration (optional)
rm -rf ~/.nekobot
```
