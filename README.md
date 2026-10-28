# Setrya-IoTMonitoring-Service

Microservicio **IoT Monitoring and Notifications** de Sentrya (SpaceUp). Recibe lecturas de dispositivos IoT (REST o MQTT), detecta valores fuera de rango, genera alertas y las notifica al usuario responsable. 

## Estructura

```
cmd/server/                 punto de entrada y wiring
api/                        especificación OpenAPI (embebida)
migrations/                 migraciones SQL (embebidas)
deployments/                docker-compose (MySQL, Mosquitto, servicio)
internal/
  domain/                   IoTDevice, Reading, Alert, VOs, servicios y repositorios (interfaces)
  application/              commands/queries, handlers y DTOs
  interfaces/rest/          controllers, resources y assemblers
  infrastructure/
    persistence/mysql/      repositorios y migrador
    mqtt/                   IoTBrokerAdapter
    notification/           NotificationDispatcherService (SMTP)
    normalizer/             ReadingNormalizerService
  config/                   configuración por variables de entorno
```

## Ejecución

```bash
docker compose -f deployments/docker-compose.yml up --build
```

Pruebas: `go test ./...`. La especificación se sirve en `GET /openapi.yaml`.

## Variables de entorno

| Variable | Default | Descripción |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Dirección HTTP |
| `MYSQL_DSN` | `sentrya:sentrya@tcp(localhost:3306)/iot_monitoring` | Conexión MySQL |
| `MQTT_BROKER_URL` | _(vacío)_ | Ej. `tcp://localhost:1883`; vacío desactiva MQTT |
| `MQTT_TOPIC` | `sentrya/+/+/readings` | Topic de suscripción |
| `MQTT_USERNAME` / `MQTT_PASSWORD` | | Credenciales del broker |
| `SMTP_HOST` | _(vacío)_ | `host:puerto`; vacío solo registra la alerta en el log |
| `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM` | | Credenciales y remitente |
| `ALERT_RECIPIENT` | | Destinatario fijo de las alertas |

## API (`/api/v1`)

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/devices` | Registra un dispositivo (nace **inactivo**) |
| PATCH | `/devices/{deviceId}/activate` · `/deactivate` | Activa / desactiva |
| GET | `/spaces/{spaceId}/devices` | Dispositivos del espacio |
| POST | `/readings` | Ingresa una lectura (`deviceId`, `metricType`, `value`, `unit`) |
| GET | `/spaces/{spaceId}/readings?limit=` | Lecturas del espacio |
| GET | `/spaces/{spaceId}/alerts?status=` | Alertas del espacio |
| GET | `/spaces/{spaceId}/monitoring-summary` | Resumen del monitoreo |
| PATCH | `/alerts/{alertId}/acknowledge` | Reconoce una alerta |
| PATCH | `/alerts/{alertId}/close` | Cierra una alerta |

Ciclo de una alerta: `created → sent → acknowledged → closed`. Una lectura de un dispositivo inexistente (404) o inactivo (409) se rechaza y no se guarda.

Payload MQTT esperado: `{"deviceId":"...","metricType":"temperature","value":38.2,"unit":"C"}`.

## Pendientes de diseño (marcados como `TODO(faltante)` en el código)

Dependen de que el resto del equipo cierre su parte; se revisan cuando estén listos.

1. **Umbrales:** el informe (US06/US07) habla de umbrales configurables pero no define dónde ni los valores por defecto. Hoy se guardan por dispositivo al registrarlo y, si faltan, se usan valores provisionales (`domain.DefaultThresholds`).
2. **Criterio de severidad:** el informe solo nombra los cuatro niveles. Se usa la desviación relativa al ancho del rango (<10 % baja, <25 % media, <50 % alta, ≥50 % crítica).
3. **Topic y payload MQTT:** pendiente de definir con Edge/Embedded.
4. **Tipos de métrica:** se asumen `temperature`, `voltage`, `sound`.
5. **Autenticación:** falta definir validación del JWT de IAM (¿gateway o servicio?) y autenticación de dispositivos.
6. **Destinatario de alertas:** el correo del dueño vive en IAM/Space Management; hoy es un valor fijo por entorno.
7. **Validación de `spaceId`** contra Space Management y **quién activa** el dispositivo (Payments).
8. **Cierre de alertas:** si se puede cerrar sin reconocer (hoy sí).
9. Los nombres de herramientas del cap. 4.2.5.4 (Entity Framework Core, Fluent API) deben actualizarse en el informe: aquí se usa Go con `database/sql`.
