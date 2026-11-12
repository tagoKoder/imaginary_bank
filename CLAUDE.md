# CLAUDE.md: Imaginary Bank

Este repositorio es la banca digital académica de un TFE sobre **autorización contextual adaptativa al riesgo y Zero Trust en microservicios** (Cognito + Cedar/Amazon Verified Permissions).

**El inventario completo y verificado del estado actual (2026-11-10) está en [`docs/estado-actual/`](docs/estado-actual/00-INDICE.md).** Léelo antes de releer el código:

| Documento | Contenido |
|---|---|
| `00-INDICE.md` | Resumen, componentes, versiones y cómo se ejecuta |
| `01-arquitectura-actual.md` | Componentes, llamadas, BD, despliegue y diagrama de confianza |
| `02-contratos-y-endpoints.md` | Endpoints REST y gRPC con la autorización real de cada uno |
| `03-flujos.md` | Secuencias de login, onboarding, cuentas, movimientos y transferencia |
| `04-identidad-y-autorizacion.md` | Cognito, JWT, propagación, autenticación entre servicios, Cedar/AVP y fallo seguro |
| `05-modelo-de-datos.md` | Tablas, titularidad, estados, importes y límites |
| `06-escenarios-TFE.md` | E1–E12: comportamiento actual y línea base |
| `07-pendientes-y-objetivo.md` | Deuda técnica y hoja de ruta priorizada (S/M/L) |

## Mapa rápido

| Componente | Ruta | Tecnología | Puerto |
|---|---|---|---|
| SPA | `imaginarybank-web/` | Angular 20 | — |
| BFF | `bff/` | Go | HTTP :8080, sesión cookie, token relay |
| identity | `micro/identity/` | Java 25 / Spring Boot 4 | gRPC :9091 (OIDC, sesiones, onboarding) |
| account | `micro/account/` | Java | gRPC :9092 (clientes, cuentas, saldos, holds) |
| ledger | `micro/ledger/` | Go | gRPC, 8082 por defecto y 9093 en Docker (libro mayor, pagos) |

- Contratos en repositorios hermanos:
  - `../bank-contracts`: protos v0.3.0, que consumen account, identity y ledger.
  - `../bank-bff-openapi`: OpenAPI v0.0.20, que consumen el BFF y la web.
  - El BFF genera sus protos desde BSR v0.0.19.
- Infraestructura: `infra/cloudformation/` (stacks 00–80) y `infra/authz/cedar/` (esquema y políticas).
- Autorización:
  - Interceptores gRPC con un registro de rutas por servicio (`*/security/authz/RouteAuthzRegistry*`, `route_registry.go`).
  - Plantillas de recurso y cliente AVP `IsAuthorizedWithToken`.

## Hechos críticos (no asumir lo contrario sin volver a verificar)

- Account **no aplica el DENY de AVP**: solo lo registra en el log (`AuthzServerInterceptor.java:269-284`).
- Identity no tiene rutas AUTHZ y usa el perfil `local` por defecto, que no tiene autorización.
- Ledger envía acciones (`Action::"ledger.*"`) que no existen en el esquema Cedar.
- No hay umbral, `auth_time`, step-up ni identidad de servicio uniforme.
- El BFF convierte todo error gRPC en 502.
- Hay contraseñas de RDS en claro en `infra/params/*/20-data.json`.

## Convenciones de trabajo

- El árbol de trabajo tenía 68 cambios sin commitear el 2026-11-10: migración de los protos a `bank-contracts`, cambios en authz y `ServiceTokenProvider`.
- Etiquetar los análisis como [VERIFICADO], [INFERIDO] o [NO ENCONTRADO] y citar `ruta:línea`.
- Ignorar `target/`, `node_modules/`, `.angular/` y el código generado (`bff/internal/**/gen`, `bank-contracts/gen`).
