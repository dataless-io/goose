# Broker Technical Design

## Overview
This package implements a lightweight, high-throughput, persistent event broker designed to replace database polling. It provides a push-based mechanism for internal services to communicate asynchronously with strong consistency guarantees where needed.

## Core Architecture

### 1. Persistence Strategy: Append-Only Log (AOL)
- **Mechanism**: All incoming events are sequentially appended to a disk file (`events.log`).
- **Format**: JSON Lines (NDJSON). Each line conforms to the `Entry` struct (Timestamp + Payload).
- **Durability**: Using `O_APPEND` ensures atomic writes at the OS level (within limits).
- **Recovery**: On startup, the broker replays the log to rebuild in-memory state if necessary (though mostly stateless).

### 2. Transport Protocol
- **Method**: HTTP/1.1 Full-Duplex.
- **Encoding**: `Transfer-Encoding: chunked`.
- **Content-Type**: `application/x-ndjson` (JSON Lines).
- **Philosophy**: stateless, firewall-friendly, leveraging existing HTTP infrastructure. Buffer-less streaming to minimize latency.

### 3. Concurrency Model: ID-Serialized Parallelism
- **Default**: Parallel execution. Workers process events concurrently.
- **Serialization Constraint**: If multiple events share the same `EntityID` (e.g., `honk_id`), they are queued and processed sequentially for that ID.
- **Implementation**: In-memory sharding or locking map based on ID hash.

### 4. Blocking Publish (Backpressure)
- **API**: `Send(stream, payload, required_consumers...)`
- **Behavior**: The call blocks until all `required_consumers` have acknowledged receipt and processing of the message.
- **Use Case**: Critical user actions (e.g., posting a tweet) where we want to guarantee persistence or immediate side-effects before responding to the user.

## Data Structures

```go
type Broker struct {
   // ...
}

type Event struct {
    ID        string          `json:"id"`
    Stream    string          `json:"stream"`
    Timestamp int64           `json:"timestamp"`
    Payload   json.RawMessage `json:"payload"`
}
```

## Future Extensibility
Designed to run as an embedded sidecar initially, but with strict boundary interfaces to allow extraction into a standalone microservice (gRPC/HTTP) without changing consumer logic.
