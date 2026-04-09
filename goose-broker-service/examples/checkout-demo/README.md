# Checkout Demo

Pequeno servicio que usa Goose Broker desde el punto de vista de producto:

- Expone `POST /checkout`.
- Publica cada checkout por `PUT /brokers/{name}/messages` en un topic dedicado.
- Requiere commit del consumidor `checkout-worker` para confirmar la peticion.
- Levanta un consumidor real en background para procesar mensajes del topic.

## Ejecutar

1. Arranca el broker:

```bash
go run ./cmd/brokerd
```

2. Arranca el demo:

```bash
go run ./examples/checkout-demo
```

3. Prueba una compra:

```bash
curl -X POST http://127.0.0.1:8091/checkout \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord-1001","amount":42.50}'
```

La respuesta incluye `commit_latency`, que refleja el tiempo real que tardo el broker en confirmar el commit requerido.
