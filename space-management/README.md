# Setrya Space Management

Microservicio Go para administrar los espacios de Sentrya. Mantiene propietario, nombre, descripción, ubicación, dimensiones y disponibilidad. El registro de dispositivos, lecturas y alertas pertenece a IoT Monitoring.

## Ejecutar localmente

1. Inicia el MySQL del compose desde esta carpeta:

   ```sh
   docker compose -f deployments/docker-compose.yml up --build
   ```

2. La API queda en `http://localhost:8081`; la especificación está en `api/openapi.yaml` y el chequeo de salud en `/health`.

El compose usa un contenedor MySQL propio en el puerto 3307 (configurable con `SPACE_MYSQL_PORT`) y el puerto HTTP 8081 (`SPACE_HTTP_PORT`). La migración se aplica al inicializar un volumen nuevo. Para instalación en una base existente, ejecuta `migrations/001_init.sql`.

## Configuración

| Variable | Default | Descripción |
|---|---|---|
| `HTTP_ADDR` | `:8081` | Dirección HTTP del servicio |
| `MYSQL_DSN` | `sentrya:sentrya@tcp(localhost:3306)/space_management?parseTime=true` | Conexión MySQL |

## API v1

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/api/v1/spaces` | Crea un espacio. El ID UUID lo genera el servicio. |
| GET | `/api/v1/spaces/{spaceId}` | Devuelve el detalle del espacio. |
| GET | `/api/v1/owners/{ownerId}/spaces` | Lista espacios de un propietario. |
| PUT | `/api/v1/spaces/{spaceId}` | Actualiza nombre, descripción, ubicación y dimensiones. |
| PATCH | `/api/v1/spaces/{spaceId}/availability` | Recibe `{ "active": true|false }` para publicar o pausar. |
| GET | `/health` | Comprueba que el proceso responde. |

Ejemplo de creación:

```json
{
  "ownerId": "user-123",
  "name": "Sala principal",
  "description": "Espacio monitoreado",
  "location": { "address": "Av. Ejemplo 123", "district": "Miraflores", "city": "Lima" },
  "dimensions": { "width": 4.5, "length": 5.0 }
}
```

La API devuelve `201 Created` y un espacio con `id`, `status`, `createdAt` y `updatedAt`.

## Integración con IoT Monitoring

IoT puede consultar `GET /api/v1/spaces/{spaceId}` antes de aceptar la asociación de un dispositivo. La respuesta incluye el propietario y estado del espacio; un espacio inexistente devuelve 404. La activación de dispositivos permanece bajo el contexto IoT/Payments según los acuerdos del proyecto.

## Seguridad e integración pendientes de decisión del equipo

IAM y el API Gateway deben validar el JWT y derivar el propietario autenticado. Mientras no exista ese contrato, `ownerId` en la petición es solo un dato de entrada y no constituye autorización. No expongas el servicio directamente a usuarios hasta aplicar esa validación. La comunicación de servicio a servicio debe restringirse a la red interna y autenticarse según la infraestructura acordada.
