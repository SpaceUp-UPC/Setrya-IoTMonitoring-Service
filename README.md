# Setrya-IoTMonitoring-Service

Microservicio **IoT Monitoring and Notifications** de Sentrya (SpaceUp). Recibe por MQTT las lecturas y eventos que publica la Edge Application, detecta condiciones anómalas, genera alertas y las notifica al usuario. También publica hacia el Edge las órdenes a actuadores, la validación de intrusiones y la configuración de umbrales.

Stack: Go (librería estándar `net/http`), MySQL y MQTT.

## Cómo encaja

```
ESP32 x3 ──HTTP──► Edge (sentrya-edge-service) ──MQTT──► broker ──► este servicio ──► MySQL
                          ▲                                 │
                          └──── commands / validation / config ◄──┘
App web ──HTTPS + JWT de IAM──► este servicio (API REST)
```

## Estructura

```
cmd/server/                 punto de entrada y wiring
api/                        especificación OpenAPI (embebida)
migrations/                 migraciones SQL (embebidas, se aplican al arrancar)
deployments/                docker-compose para desarrollo (MySQL, Mosquitto, servicio)
render.yaml                 blueprint de despliegue en Render
internal/
  domain/                   IoTDevice, Reading, Alert, VOs, servicios y repositorios (interfaces)
  application/              commands/queries, handlers y DTOs
  interfaces/rest/          controllers, resources, assemblers, JWT y CORS
  infrastructure/
    persistence/mysql/      repositorios y migrador
    mqtt/                   IoTBrokerAdapter (suscripción y publicación)
    notification/           NotificationDispatcherService (SMTP)
    normalizer/             ReadingNormalizerService
  config/                   configuración por variables de entorno
```

## Ejecución local

```bash
docker compose -f deployments/docker-compose.yml up --build
```

Levanta MySQL, Mosquitto y el servicio en `http://localhost:8080`, con la autenticación desactivada (`AUTH_DISABLED=true`). Si un puerto está ocupado, cámbialo con `MYSQL_PORT`, `MQTT_PORT` o `HTTP_PORT`.

Pruebas: `go test ./...`. La especificación se sirve en `GET /openapi.yaml`.

## Variables de entorno

| Variable | Default | Descripción |
|---|---|---|
| `PORT` / `HTTP_ADDR` | `:8080` | Puerto HTTP. Render define `PORT` |
| `MYSQL_DSN` | local | `mysql://usuario:clave@host:puerto/base` o el formato del driver |
| `MYSQL_TLS` | _(vacío)_ | `true`, `skip-verify` (CA propia del proveedor) o `preferred` |
| `MQTT_BROKER_URL` | _(vacío)_ | `tcp://host:1883` o `ssl://host:8883`. Vacío desactiva MQTT |
| `MQTT_TOPIC` | `sentrya/+/+/readings` | Tópico de suscripción |
| `MQTT_CLIENT_ID` | `sentrya-iot-monitoring` | Fijo, para que el broker conserve la sesión |
| `MQTT_USERNAME` / `MQTT_PASSWORD` | | Credenciales del broker |
| `JWT_SECRET` | **obligatorio** | El mismo `authorization.jwt.secret` de IAM |
| `AUTH_DISABLED` | `false` | `true` solo en desarrollo: abre la API sin token |
| `CORS_ALLOWED_ORIGINS` | `*` | Orígenes de la app web, separados por coma |
| `SMTP_HOST` | _(vacío)_ | `host:puerto`. Vacío: la alerta solo se registra en el log |
| `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM` | | Credenciales y remitente |
| `ALERT_RECIPIENT` | | Destinatario fijo de las alertas |

El servicio no arranca sin `JWT_SECRET`, salvo que se indique `AUTH_DISABLED=true`.

## Despliegue en Render

El repositorio incluye `render.yaml`. Render construye la imagen con el `Dockerfile`, usa `/health` como health check y vuelve a desplegar con cada push a `main`.

Antes de desplegar hacen falta dos servicios externos, porque Render no los ofrece:

1. **MySQL 8 gestionado** (por ejemplo Aiven, Railway o Clever Cloud). Se necesita su URL de conexión.
2. **Broker MQTT en la nube** con TLS y usuario (por ejemplo HiveMQ Cloud o EMQX Cloud). Debe ser el mismo broker al que apunta el Edge, porque la laptop del Edge no puede alcanzar un broker dentro de Render.

Pasos:

1. En Render: **New → Blueprint**, elegir este repositorio.
2. Completar las variables que pide el panel:
   - `MYSQL_DSN`: `mysql://usuario:clave@host:puerto/base`. Si la conexión falla por certificado, cambiar `MYSQL_TLS` a `skip-verify`.
   - `MQTT_BROKER_URL` (`ssl://host:8883`), `MQTT_USERNAME`, `MQTT_PASSWORD`.
   - `JWT_SECRET`: el secreto de IAM.
   - `CORS_ALLOWED_ORIGINS`: la URL de la app web.
3. Al terminar, comprobar `https://<servicio>.onrender.com/health`.
4. Registrar los dispositivos (ver abajo) y configurar el Edge con el mismo broker y los mismos `deviceId`.

**Plan gratuito:** Render suspende el servicio tras 15 minutos sin tráfico HTTP, y mientras está suspendido no recibe MQTT. La sesión MQTT es persistente, así que el broker guarda las lecturas QoS 1 y las entrega al despertar, pero las alertas llegan tarde. Para la demo conviene un plan de pago o un ping periódico a `/health`.

## Registro de dispositivos

El Edge publica en nombre de las ESP32 y necesita conocer cada `deviceId` de antemano, así que se registran con **ids acordados**, los mismos que van en `edge/config/devices.yaml`:

```bash
curl -X POST https://<servicio>/api/v1/devices \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"space1-temperature","spaceId":"space1","name":"Temperatura","active":true,
       "thresholds":[{"metricType":"temperature","minValue":-40,"maxValue":28,"unit":"C"}]}'
```

Dispositivos que espera el Edge: `temperature`, `humidity`, `intrusion` y `escalation`. Los umbrales son opcionales; sin ellos se usan los valores por defecto (`domain.DefaultThresholds`). El `spaceId` debe ser el `SPACE_ID` del Edge.

## Contrato MQTT

**Edge → nube.** Tópico `sentrya/{spaceId}/{deviceId}/readings`, QoS 1:

```json
{"deviceId":"space1-temperature","metricType":"temperature","value":29.1,"unit":"C",
 "readingId":"<uuid>","measuredAt":"2026-11-09T05:39:45Z","localState":"WARNING"}
```

| `metricType` | `value` / `unit` | Alerta |
|---|---|---|
| `temperature` | °C / `C` | fuera de umbral o `localState` WARNING (media) / CRITICAL (crítica) |
| `humidity` | % / `%` | fuera de umbral |
| `intrusion` | `1` / `event` | siempre, severidad alta. Lleva `eventId` y `evidenceRef` |
| `escalation` | `1` / `event` | siempre, severidad crítica. Lleva `reason` |
| `device_error` | `1` / `event` | severidad media |

Reglas:

- `readingId` hace la ingesta **idempotente**: un reenvío del outbox del Edge no duplica la lectura ni la alerta.
- Mientras una condición dura, no se crea una alerta por lectura: solo si no hay otra abierta de igual o mayor severidad para ese dispositivo y métrica. Intrusiones y escalamientos siempre generan alerta.
- Se rechazan (y no se guardan) las lecturas de un dispositivo inexistente o inactivo.
- Los campos desconocidos se ignoran.

**Nube → Edge.** Se publican al llamar a la API:

| Tópico | Origen | Payload |
|---|---|---|
| `sentrya/{spaceId}/{target}/commands` | `POST /spaces/{spaceId}/commands/{target}` | `{"commandId","action","params"}` |
| `sentrya/{spaceId}/security/validation` | `POST /alerts/{alertId}/validation` | `{"eventId","decision"}` |
| `sentrya/{spaceId}/config` | `PUT /spaces/{spaceId}/edge-config` | árbol de umbrales (sin retain) |

Si el broker no está disponible, estas rutas responden `503` y no cambian nada.

## API (`/api/v1`)

Todas requieren `Authorization: Bearer <JWT de IAM>`. `/health` y `/openapi.yaml` son públicas.

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/devices` | Registra un dispositivo (`id` y `active` opcionales) |
| PATCH | `/devices/{deviceId}/activate` · `/deactivate` | Activa / desactiva |
| GET | `/spaces/{spaceId}/devices` | Dispositivos del espacio |
| POST | `/readings` | Ingresa una lectura por HTTP (mismo cuerpo que el payload MQTT) |
| GET | `/spaces/{spaceId}/readings?limit=` | Lecturas del espacio |
| GET | `/spaces/{spaceId}/alerts?status=` | Alertas del espacio |
| GET | `/spaces/{spaceId}/monitoring-summary` | Resumen del monitoreo |
| PATCH | `/alerts/{alertId}/acknowledge` | Reconoce una alerta |
| PATCH | `/alerts/{alertId}/close` | Cierra una alerta |
| POST | `/alerts/{alertId}/validation` | Confirma o rechaza una intrusión (`{"decision":"confirm"\|"reject"}`) |
| POST | `/spaces/{spaceId}/commands/{target}` | Orden a un actuador (`valve`, `lock`, `humidifier` o un `deviceId`) |
| PUT | `/spaces/{spaceId}/edge-config` | Envía umbrales al Edge |

Ciclo de una alerta: `created → sent → acknowledged → closed`.

## Pendientes (marcados como `TODO(faltante)` en el código)

1. **Autorización por espacio:** el JWT de IAM solo identifica al usuario. Cualquier usuario autenticado puede consultar o enviar órdenes a cualquier espacio hasta que Space Management exponga quién es dueño de cada uno.
2. **Validación de `spaceId`** contra Space Management (su servicio aún no está publicado).
3. **Destinatario de las alertas:** hoy es un correo fijo (`ALERT_RECIPIENT`); falta obtener el del dueño del espacio.
4. **Evidencia de intrusión:** `evidenceRef` es una ruta local del Edge; la nube guarda la referencia pero no puede mostrar la foto.
5. **Cierre de alertas:** si se puede cerrar sin reconocer (hoy sí).
6. El informe (cap. 4.2.5.4) menciona Entity Framework Core; debe actualizarse a Go con `database/sql`.
