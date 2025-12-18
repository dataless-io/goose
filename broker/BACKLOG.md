# Broker Backlog

## Actual features
- [x] **Append-Only Log Persistence**: Write events to disk sequentially using standard file I/O.
- [x] **HTTP Stream Transport**: Implement endpoint `/listen` supporting `Transfer-Encoding: chunked` and JSONL format.
- [x] **Blocking Send**: Implement `Send` method that blocks until specific subscribers ACK.
- [x] **ID-based Serialized Concurrency**: Ensure events with same ID are processed in order, while allowing parallelism for others.
- [x] **In-Memory Subscriber Map**: Simple management of active channels/connections.
- [x] **Persistent ACKs**: Write acknowledgment records to the Append-Only Log.
- [x] **Standalone Extraction**: Refactor into a separate binary/service (microservice).
- [x] **Offset/Cursor Management**: Allow consumers to resume from a specific timestamp/offset explicitly.

## Next features
- [ ] **Compaction/Rotation**: Log rotation and old event cleanup.
- [ ] **Consumer Groups**: Load balancing across multiple instances of the same consumer.

## Will not do these features
- [ ] **Redis/NATS Integration**: No external dependencies allowed.
- [ ] **WebSockets**: Avoiding protocol overhead/complexity; sticking to HTTP streaming.
- [ ] **Binary Protocol**: Sticking to JSON for debuggability and simplicity.
