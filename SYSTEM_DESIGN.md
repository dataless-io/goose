# System Design & Architecture Analysis - Goose

Este documento detalla la arquitectura, abstracciones y patrones identificados en el proyecto `goose`.

## 1. Visión General de la Arquitectura

El sistema sigue una arquitectura de **"Modular Monolith"** con una fuerte orientación a eventos (**Event-Driven Architecture**). Aunque el código corre principalmente en un solo binario (`cmd/goose`), los componentes están desacoplados a través de un bus de eventos persistente basado en la propia base de datos.

### Componentes Principales
1.  **API Gateway / Web Server**: Gestiona las peticiones HTTP (REST-ish). Utiliza la librería `box` para enrutamiento e inyección de dependencias.
2.  **Event Bus (Streams)**: Sistema de cola de mensajes implementado sobre la base de datos (polling).
3.  **Workers (Consumers)**: Go-routines iniciadas en el `Bootstrap` que consumen eventos del bus para realizar tareas asíncronas (persistencia, notificaciones, fan-out).
4.  **Storage Layer (InceptionDB)**: Base de datos documental (JSON) accedida vía HTTP. Actúa como *Source of Truth* y como *Message Broker*.

## 2. Abstractions Core

### Web Framework (`box`)
*   **Abstracción**: `box.B` (Builder), `box.Resource`, `box.Action`.
*   **Patrón**: Interceptors (Middleware) para gestión transversal (Auth, Logging, DI).
*   **Dependency Injection**: Inyección manual a través de interceptores (`InjectInceptionClient`, `InjectStreams`). Los handlers declaran sus dependencias en la firma de la función.

### Data Access (`inceptiondb`)
*   **Abstracción**: `Client`.
*   **Modelo de Datos**: Documental sin esquema estricto (JSON blobs).
*   **Schema Enforcement**: Validada implícitamente al arranque mediante `EnsureIndex`.
*   **Comunicación**: HTTP REST client. No hay driver nativo de bajo nivel, todo es JSON over HTTP.

### Async Processing (`streams`)
*   **Abstracción**: `Streams` (Send/Receive).
*   **Implementación**:
    *   **Productor**: Inserta un documento en una colección `streams.stream.<nombre>`.
    *   **Consumidor**: Bucle infinito (`for !stop`) que hace polling (`Find`) a la colección buscando nuevos registros basados en un timestamp (`counter.Last`).
    *   **Reliability**: Mantiene un cursor (checkpoint) en `streams.counters`.

## 3. Patrones de Diseño Identificados

### Fan-out on Write (Push Model)
Para la funcionalidad de Timeline (Twitter-like):
*   Cuando se crea un "honk" (tweet), no se escribe directamente en las timelines.
*   Se emite un evento `honk_create`.
*   Un worker (`follow_user`) busca **todos** los seguidores del autor.
*   Itera sobre cada seguidor e inserta una copia/referencia del honk en la colección `user_honks`.
*   **Consecuencia**: Lectura del timeline es O(1) (muy rápida), pero escritura es O(N) donde N es el número de seguidores.

### Event Sourcing (Lite)
El sistema confía en el log de eventos (`honk_create`) como disparador de efectos secundarios. Si se borra la proyección (`user_honks`), teóricamente podría reconstruirse reprocesando el stream (siempre que se resetearan los contadores), aunque no es un Event Sourcing purista.

### Polling Consumer
En lugar de usar websockets o conexiones persistentes TCP para los eventos internos, los workers sondean la base de datos cada segundo (`time.Sleep`).

## 4. Análisis Crítico

### Puntos Fuertes
*   **Simplicidad**: El modelo mental es muy sencillo. Todo (datos y colas) vive en el mismo sitio (InceptionDB).
*   **Desacoplamiento**: La API de publicación (`/publish`) es muy rápida porque solo valida y emite un evento. El trabajo pesado es asíncrono.
*   **Resiliencia**: Si el proceso muere, al reiniciar continúa leyendo eventos desde donde se quedó (gracias a los cursores persistentes).

### Puntos Débiles & Riesgos
*   **Escalabilidad del Fan-out**: El bucle de `follow_user` en `bootstrap.go` es síncrono y secuencial.
    *   *Riesgo*: Si un usuario tiene 100k seguidores, la publicación tardará mucho en propagarse y bloqueará ese worker.
*   **Latencia por Polling**: Existe un retraso garantizado de hasta 1 segundo (hardcoded sleep) entre que sucede un evento y se procesa.
*   **Database as a Queue**: Usar la base de datos principal como cola de mensajes genera carga de lectura/escritura constante (polling), incluso cuando el sistema está en reposo ("Empty spins").
*   **Single Point of Failure**: `bootstrap.go` define todos los consumers en el mismo proceso que el servidor web. Si escala el tráfico web y se despliegan N réplicas, se multiplicarán los consumers procesando los mismos eventos.
*   **Race Conditions en Escalado Horizontal**: `streams.go` usa cursores globales en DB. Si hay múltiples réplicas de `goose`, **competirán** por procesar los eventos y actualizar el contador. Esto causará condiciones de carrera severas (eventos procesados múltiples veces o saltados).

## 5. Propuestas de Mejora

### 1. Separar Consumers de API
Mover los `go func()` del `Bootstrap` a un comando separado (ej: `cmd/worker`). Esto permite escalar la API independientemente de los procesadores de fondo.

### 2. Implementar Locking o Partitioning en Streams
Para evitar que múltiples réplicas procesen el mismo evento, se necesita un mecanismo de coordinación (e.g., "Leader Election" o particionado de streams por ID de usuario).

### 3. Optimizar Fan-out (Hybrid Model)
Para usuarios con muchos seguidores ("celebrities"), no hacer fan-out a todas las timelines.
*   Al leer el timeline, mezclar en tiempo real los tweets de las celebridades que sigues con tus `user_honks` pre-calculados.

### 4. Implementar Broker de Eventos Nativo (Custom)
Se diseñará un nuevo componente `Broker` con las siguientes características clave para reemplazar la dependencia de InceptionDB como cola:

*   **Persistencia (Append-only Log)**:
    *   Los eventos se escribirán secuencialmente en un archivo en disco (`events.log`).
    *   Esto garantiza durabilidad y recuperación ante fallos sin la complejidad de una DB completa.
    *   *Simplicidad*: Escritura O(1) al final del archivo.

*   **Publicación Síncrona / Bloqueante (Backpressure)**:
    *   La firma del método `Send` permitirá especificar consumidores críticos que deben confirmar recepción antes de retornar:
        `broker.Send(stream, payload, "insert_honk", "calculate_links")`
    *   El productor se bloqueará hasta que los consumidores indicados hayan hecho ACK (commit) del evento.
    *   Esto garantiza consistencia fuerte para flujos críticos (ej: no confirmar un tweet al usuario hasta que esté persistido).

*   **Diseño Modular y Extraíble**:
    *   El Broker se desarrollará como un paquete independiente con interfaces claras (gRPC o HTTP interno).
    *   Inicialmente correrá embebido en el monolito (`sidecar` in-process), pero estará desacoplado para poder extraerse a un microservicio independiente en el futuro sin reescribir la lógica de negocio.

*   **Mecanismo de Push**:
    *   Los consumidores se suscribirán al Broker (vía canales en intra-process o TCP/Socket si se extrae).
    *   Elimina la latencia del polling actual.

*   **Transporte: HTTP Full-Duplex (JSON Lines)**:
    *   Se utilizará HTTP con `Transfer-Encoding: chunked` (streaming) enviando objetos JSON separados por saltos de línea (NDJSON).
    *   **Pros**:
        *   Aprovecha infraestructuras HTTP existentes (Balanceadores, Proxies, Auth Headers).
        *   Menor complejidad de implementación (handshake) que WebSockets.
        *   Firewall-friendly.
    *   **Cons**:
        *   Overhead de cabeceras HTTP por conexión.
        *   Algunos proxies intermedios pueden hacer buffering si no se configuran bien (rompiendo el real-time).

*   **Concurrencia: Paralelismo Serializado por ID**:
    *   Los workers procesarán mensajes en paralelo, *excepto* cuando coincida el ID de la entidad (e.g. `honk_id`).
    *   Si llegan eventos con el mismo ID, se encolan en memoria y se procesan secuencialmente.
    *   **Pros**:
        *   Maximiza el throughput al no bloquear todo el stream por un solo "hot entity".
        *   Garantiza orden causal estricto para una misma entidad (evita race conditions de actualizaciones).
    *   **Cons**:
        *   Complejidad alta en la gestión de locks/colas en memoria.
        *   Riesgo de uso de memoria si se acumulan eventos para un mismo ID bloqueado.

### 5. Estructura de Directorios (Refactor)
Mover la lógica de negocio que vive en `bootstrap.go` (los consumers) a un paquete `internal/workers` o `internal/usecases`. El `bootstrap.go` tiene demasiada responsabilidad (conocimiento de índices, lógica de followers, lógica de notificaciones).
