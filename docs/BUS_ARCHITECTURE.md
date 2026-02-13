# Message Bus Architecture

The message bus is the central routing system for messages between channels (Telegram, Discord, etc.) and the agent. It supports multiple backend implementations for different deployment scenarios.

## Architecture

```
┌─────────────┐
│   Channel   │ ──inbound──> Bus ──> Agent
│  (Telegram) │              │
│             │ <─outbound── Bus <── Agent
└─────────────┘
```

## Supported Backends

### 1. Local Bus (Default)

**Best for:** Single-node deployments, development, testing

**Implementation:** Uses Go channels for in-process message routing

**Configuration:**
```json
{
  "bus": {
    "type": "local"
  },
  "gateway": {
    "message_queue_size": 100
  }
}
```

**Features:**
- ✅ Zero external dependencies
- ✅ Lowest latency
- ✅ Simple deployment
- ❌ Single process only (no horizontal scaling)

### 2. Redis Bus

**Best for:** Distributed deployments, horizontal scaling, high availability

**Implementation:** Uses Redis pub/sub for inter-process communication

**Configuration:**
```json
{
  "bus": {
    "type": "redis",
    "redis_addr": "localhost:6379",
    "redis_password": "",
    "redis_db": 0,
    "redis_prefix": "nekobot:bus:"
  }
}
```

**Features:**
- ✅ Multiple gateway instances
- ✅ Horizontal scaling
- ✅ Process isolation
- ✅ Network-level message routing
- ❌ Requires Redis server
- ❌ Slightly higher latency

## Message Flow

### Inbound Messages (Channel → Agent)

1. User sends message to Telegram
2. Telegram channel receives update
3. Channel calls `bus.SendInbound(msg)`
4. **Local Bus:** Enqueues to Go channel → Handler
5. **Redis Bus:** Publishes to `nekobot:bus:inbound:telegram` → Subscribers receive → Handler
6. Handler (agent) processes message

### Outbound Messages (Agent → Channel)

1. Agent generates response
2. Agent calls `bus.SendOutbound(msg)`
3. **Local Bus:** Enqueues to Go channel → Channel handler
4. **Redis Bus:** Publishes to `nekobot:bus:outbound:telegram` → Channel receives
5. Channel sends message to user

## Handler Registration

Channels register handlers with the bus to receive messages:

```go
// Register handler for Telegram channel
bus.RegisterHandler("telegram", func(ctx context.Context, msg *bus.Message) error {
    return telegramChannel.SendMessage(ctx, msg)
})
```

## Message Structure

```go
type Message struct {
    ID        string      // Unique message ID (e.g., "telegram:123456")
    ChannelID string      // Channel identifier (e.g., "telegram")
    SessionID string      // Session key (e.g., "telegram:chatid")
    UserID    string      // User identifier
    Username  string      // User display name
    Type      MessageType // text, image, audio, video, file, location, command
    Content   string      // Message content
    Data      map[string]interface{} // Additional metadata
    Timestamp time.Time   // Message timestamp
    ReplyTo   string      // Reply reference
}
```

## Deployment Scenarios

### Scenario 1: Single Gateway (Local Bus)

```
┌──────────────────────────────────────┐
│         Nekobot Gateway              │
│  ┌──────────┐      ┌──────────┐     │
│  │ Telegram │◄────►│LocalBus │◄────►│ Agent
│  │ Discord  │      │         │      │
│  └──────────┘      └──────────┘     │
└──────────────────────────────────────┘
```

**Configuration:**
```bash
# Default - no special config needed
nekobot gateway
```

### Scenario 2: Multiple Gateways (Redis Bus)

```
┌─────────────┐          ┌─────────┐          ┌──────────────┐
│  Gateway 1  │◄────────►│  Redis  │◄────────►│   Gateway 2  │
│  Telegram   │  pub/sub │  Bus    │  pub/sub │   Discord    │
└─────────────┘          └─────────┘          └──────────────┘
                              ▲
                              │
                         ┌────┴─────┐
                         │  Agent   │
                         │ Instance │
                         └──────────┘
```

**Configuration:**
```json
{
  "bus": {
    "type": "redis",
    "redis_addr": "redis.example.com:6379"
  }
}
```

**Start multiple instances:**
```bash
# Instance 1 - Telegram
nekobot gateway

# Instance 2 - Discord
nekobot gateway

# Instance 3 - Agent processing
nekobot gateway
```

### Scenario 3: Load Balanced (Redis Bus + Multiple Agents)

```
           ┌─────────────┐
           │  Redis Bus  │
           └──────┬──────┘
                  │
      ┌───────────┼───────────┐
      ▼           ▼           ▼
┌──────────┐ ┌──────────┐ ┌──────────┐
│ Agent 1  │ │ Agent 2  │ │ Agent 3  │
└──────────┘ └──────────┘ └──────────┘
```

Redis pub/sub naturally load-balances across multiple subscribers.

## Future Extensions

The bus interface can be extended to support additional backends:

### NATS
```go
type NATSBus struct {
    conn *nats.Conn
    // ...
}
```

### MQTT
```go
type MQTTBus struct {
    client mqtt.Client
    // ...
}
```

### Kafka
```go
type KafkaBus struct {
    producer sarama.SyncProducer
    consumer sarama.ConsumerGroup
    // ...
}
```

Simply implement the `Bus` interface and add to the factory.

## Metrics

Both implementations track metrics:

```go
metrics := bus.GetMetrics()
// Returns:
// {
//   "messages_in": 1234,
//   "messages_out": 1100,
//   "errors": 5
// }
```

## Best Practices

1. **Development:** Use local bus for simplicity
2. **Production (single server):** Use local bus for best performance
3. **Production (distributed):** Use Redis bus for horizontal scaling
4. **High throughput:** Consider NATS or Kafka for millions of messages
5. **Testing:** Local bus is easier to test and debug

## Implementation Details

### Local Bus
- Uses buffered Go channels (default: 100)
- Synchronous handler execution
- In-process only
- ~1μs latency

### Redis Bus
- Uses Redis pub/sub
- Pattern subscription: `nekobot:bus:*`
- JSON serialization
- ~1-5ms latency (local Redis)
- ~10-50ms latency (remote Redis)

## Migration

### From Local to Redis

1. Install Redis server
2. Update configuration:
   ```json
   {
     "bus": {
       "type": "redis",
       "redis_addr": "localhost:6379"
     }
   }
   ```
3. Restart gateway - no code changes needed!

### From Redis to Local

1. Update configuration:
   ```json
   {
     "bus": {
       "type": "local"
     }
   }
   ```
2. Restart gateway
3. Remove Redis if no longer needed
