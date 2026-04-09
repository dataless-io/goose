# Goose Broker Service

Servicio independiente de broker de eventos (tipo Kafka ligero) extraido de `goose/broker`.

## Caracteristicas

- Persistencia append-only en disco (NDJSON).
- Rotacion de logs por tamano y retencion configurable.
- Multiples brokers por nombre (`/brokers/{name}`).
- Publicacion con consumidores requeridos y ack en log.
- Publicacion por `PUT` con bloqueo hasta commit de consumidores requeridos.
- Suscripcion por stream via HTTP chunked (`application/x-ndjson`).
- Endpoints de salud y arranque graceful para despliegue en produccion.
- Consola web embebida para operar brokers desde navegador.
- CRUD de topics por broker (`create/update/delete/list/get`).
- Metricas runtime por broker y por topic.

## Ejecutar local

```bash
go run ./cmd/brokerd
```

Por defecto escucha en `:8081` y guarda logs en `./data`.

## Variables de entorno

El servicio usa `github.com/fulldump/goconfig` para leer configuracion desde flags, env y `config.json`.

Puedes partir de `config.example.json` como plantilla.

Variables de entorno soportadas (segun el nombre del campo):

- `HTTPADDR` (default `:8081`)
- `PERSISTENCEDIR` (default `./data`)
- `DEFAULTBROKERNAME` (default `default`)
- `APIKEY` (default vacio, si se define protege operaciones de escritura)
- `STATICS` (default vacio, usa UI embebida)
- `HTTPREADTIMEOUT` (default `10s`)
- `HTTPREADHEADERTIMEOUT` (default `5s`)
- `HTTPWRITETIMEOUT` (default `0`, sin limite)
- `HTTPIDLETIMEOUT` (default `60s`)
- `HTTPSHUTDOWNTIMEOUT` (default `15s`)
- `MAXLOGSIZEBYTES` (default `10485760`)
- `MAXRETAINEDLOGS` (default `5`)
- `UNSAFEBATCHWRITES` (default `false`)
- `SYNCONWRITE` (default `false`)

## API principal

- `GET /healthz`
- `GET /readyz`
- `GET /brokers`
- `POST /brokers`
- `GET /brokers/{name}`
- `GET /brokers/details`
- `GET /brokers/{name}/settings`
- `PUT|PATCH /brokers/{name}/settings`
- `DELETE /brokers/{name}`
- `GET /brokers/{name}/metrics`
- `GET /brokers/{name}/topics`
- `POST /brokers/{name}/topics`
- `GET /brokers/{name}/topics/{topic}`
- `PUT|PATCH /brokers/{name}/topics/{topic}`
- `DELETE /brokers/{name}/topics/{topic}`
- `POST /brokers/{name}/publish`
- `PUT /brokers/{name}/messages` (bloquea hasta `required_consumers` ack)
- `GET /brokers/{name}/subscribe?id=consumer-1&from_id=<event_id>&stream=<topic>`

Ejemplo `PUT` con bloqueo por commit:

```bash
curl -X PUT http://localhost:8081/brokers/default/messages \
  -H 'Content-Type: application/json' \
  -d '{
    "stream": "timeline",
    "payload": {"id": "msg-1", "body": "hola"},
    "required_consumers": ["fanout-worker", "search-indexer"]
  }'
```

Si alguno de esos consumidores no esta suscrito, la operacion falla con error de conflicto.

## UI

La consola web embebida (`/`) permite:

- Gestionar brokers.
- Editar metadata de broker (display name, descripcion, disable/enable).
- Gestionar topics (crear, editar, deshabilitar y eliminar).
- Publicar por `PUT /messages` con `required_consumers`.
- Consumir mensajes en vivo con filtro por topic.
- Abrir multiples sesiones de consumo concurrentes.
- Consultar metricas runtime del broker.

## Producto de ejemplo

Hay un mini-servicio funcional en `examples/checkout-demo` que usa el broker como columna vertebral de checkouts.

```bash
go run ./examples/checkout-demo
```

Luego envia:

```bash
curl -X POST http://127.0.0.1:8091/checkout -H 'Content-Type: application/json' -d '{"order_id":"ord-1001","amount":42.50}'
```

Devuelve la `commit_latency` real de la publicacion bloqueante.

## Benchmarks comparativos

Incluye benchs para Goose, NATS y Kafka en `bench/comparison_bench_test.go`.

Ejecutar Goose local:

```bash
go test -bench BenchmarkGooseHTTP_Publish -run ^$ ./bench
```

Ultima referencia local (maquina de desarrollo):

- `BenchmarkGooseHTTP_Publish-20  30801  36971 ns/op`

Comparar con NATS (si tienes servidor):

```bash
NATS_URL=nats://127.0.0.1:4222 go test -bench BenchmarkNATS_Publish -run ^$ ./bench
```

Comparar con Kafka (si tienes cluster):

```bash
KAFKA_BROKERS=127.0.0.1:9092 KAFKA_TOPIC=goose-bench go test -bench BenchmarkKafka_Publish -run ^$ ./bench
```

## Produccion

1. Ejecutar con volumen persistente montado en `BROKER_PERSISTENCE_DIR`.
2. Definir `BROKER_SYNC_ON_WRITE=true` para durabilidad fuerte (tradeoff de throughput).
3. Usar probes sobre `/healthz` y `/readyz`.
4. Exponer el servicio tras balanceador/proxy que soporte streaming chunked.

## Docker

```bash
docker build -t goose-broker-service:latest .
docker run --rm -p 8081:8081 -e BROKER_PERSISTENCE_DIR=/var/lib/goose-broker -v $(pwd)/data:/var/lib/goose-broker goose-broker-service:latest
```
