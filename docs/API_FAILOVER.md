# API Failover & Rotation

Nekobot supports intelligent API key rotation and failover to improve reliability and avoid rate limits.

## Features

### 1. Multiple API Key Profiles
Configure multiple API keys per provider with different priorities:

```json
{
  "providers": {
    "anthropic": {
      "rotation": {
        "enabled": true,
        "strategy": "round_robin",
        "cooldown": "5m"
      },
      "profiles": {
        "primary": {
          "api_key": "sk-ant-xxx-primary",
          "priority": 1
        },
        "secondary": {
          "api_key": "sk-ant-xxx-secondary",
          "priority": 2
        },
        "backup": {
          "api_key": "sk-ant-xxx-backup",
          "priority": 3
        }
      }
    }
  }
}
```

### 2. Rotation Strategies

**Round Robin** (`round_robin`)
- Cycles through profiles in order
- Distributes load evenly across all profiles
- Default strategy

**Least Used** (`least_used`)
- Selects the profile with lowest request count
- Balances usage across profiles
- Useful for quota management

**Random** (`random`)
- Randomly selects from available profiles
- Simple load distribution

### 3. Intelligent Failover

Automatic error classification:
- **Authentication Errors** (401, 403) → Put profile on cooldown
- **Rate Limits** (429) → Cooldown and switch to next profile
- **Billing Issues** → Cooldown affected profile
- **Network Errors** → Retry with same or different profile
- **Server Errors** (5xx) → Retry without cooldown

### 4. Cooldown Mechanism

When a profile fails with a retriable error, it's put on cooldown:
- Configurable cooldown duration (e.g., "5m", "30s", "1h")
- Profile becomes unavailable during cooldown
- Automatic rotation to next available profile
- Cooldown clears after duration expires

## Configuration

### Basic Configuration (Single API Key)

```json
{
  "providers": {
    "anthropic": {
      "api_key": "sk-ant-xxx"
    }
  }
}
```

### Rotation Enabled (Multiple Profiles)

```json
{
  "providers": {
    "anthropic": {
      "rotation": {
        "enabled": true,
        "strategy": "round_robin",
        "cooldown": "5m"
      },
      "profiles": {
        "profile1": {
          "api_key": "sk-ant-xxx-1",
          "priority": 1
        },
        "profile2": {
          "api_key": "sk-ant-xxx-2",
          "priority": 2
        }
      }
    }
  }
}
```

## Usage in Code

```go
import (
    "nekobot/pkg/config"
    "nekobot/pkg/logger"
    "nekobot/pkg/providers"
)

// Load configuration
cfg, err := config.Load("config.json")
if err != nil {
    log.Fatal(err)
}

// Create rotation manager from config
log, _ := logger.New(&logger.Config{Level: logger.LevelInfo})
rotationMgr, err := providers.CreateRotationManagerFromConfig(log, cfg.Providers.Anthropic)
if err != nil {
    log.Fatal(err)
}

// Get next available profile
profile, err := rotationMgr.GetNextProfile()
if err != nil {
    log.Fatal(err)
}

// Use the API key
fmt.Printf("Using profile: %s\n", profile.Name)

// After request completes
if err := requestError; err != nil {
    // Handle error and potentially cooldown
    rotationMgr.HandleError(profile, err, httpStatusCode)
} else {
    // Record success
    rotationMgr.RecordSuccess(profile)
}

// Check rotation status
statuses := rotationMgr.GetStatus()
for _, status := range statuses {
    fmt.Printf("Profile %s: available=%v, requests=%d\n",
        status.Name, status.Available, status.RequestCount)
}
```

## Error Classification

The system automatically classifies errors and determines appropriate actions:

| Error Type | Examples | Action |
|------------|----------|--------|
| Auth | 401, 403, "invalid api key" | Cooldown |
| Rate Limit | 429, "too many requests" | Cooldown |
| Billing | "quota exceeded", "payment required" | Cooldown |
| Network | Connection timeout, DNS error | Retry |
| Server | 500, 502, 503 | Retry |

## Best Practices

1. **Use Multiple Profiles**: Configure at least 2-3 profiles for better reliability
2. **Set Appropriate Cooldown**: 5-10 minutes works well for most cases
3. **Monitor Usage**: Check profile status regularly to identify issues
4. **Strategy Selection**:
   - Use `round_robin` for general use
   - Use `least_used` when you have quota limits
   - Use `random` for simple load distribution
5. **Priority**: Set lower priority numbers for preferred profiles

## Example Scenarios

### Scenario 1: Rate Limit Hit

```
1. Request to primary profile
2. Receives 429 (rate limit)
3. Primary put on 5min cooldown
4. Automatically switches to secondary
5. Request succeeds
6. After 5min, primary becomes available again
```

### Scenario 2: Authentication Failure

```
1. Request to profile with invalid key
2. Receives 401 (unauthorized)
3. Profile put on cooldown
4. Switches to next available profile
5. Logs warning about invalid key
```

### Scenario 3: All Profiles on Cooldown

```
1. All profiles hit rate limits
2. All on cooldown
3. GetNextProfile() returns error
4. Caller can either wait or fail gracefully
```

## Monitoring

Check profile status anytime:

```go
statuses := rotationMgr.GetStatus()
for _, s := range statuses {
    fmt.Printf("Profile: %s\n", s.Name)
    fmt.Printf("  Available: %v\n", s.Available)
    fmt.Printf("  Requests: %d\n", s.RequestCount)
    if !s.Available {
        fmt.Printf("  Cooldown Until: %v\n", s.CooldownUntil)
        if s.LastError != nil {
            fmt.Printf("  Last Error: %s\n", s.LastError.Message)
        }
    }
}
```

## Troubleshooting

**Problem**: All profiles on cooldown

**Solution**:
- Increase cooldown duration
- Add more profiles
- Check rate limits
- Verify API keys are valid

**Problem**: Same profile always selected

**Solution**:
- Enable rotation: `"enabled": true`
- Verify multiple profiles configured
- Check profile priorities

**Problem**: Excessive API calls

**Solution**:
- Use `least_used` strategy
- Monitor request counts
- Implement request caching
