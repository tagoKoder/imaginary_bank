# Imaginary Bank: inventario del estado actual

> **Fecha:** 2026-11-10.
> **Alcance:** árbol de trabajo actual de los tres repositorios: `imaginary_bank`, `bank-contracts` y `bank-bff-openapi`.
> **Método:** auditoría estática de solo lectura. No se han ejecutado builds, tests ni la aplicación.
> **Árbol de trabajo:** `imaginary_bank` tiene 68 cambios sin commitear (40 modificados, 26 borrados y 2 sin seguimiento). Se han borrado los `.proto` locales y el `genproto` de ledger para pasar a `bank-contracts`. Además hay cambios en los interceptores de autorización, el paquete nuevo `micro/account/.../infra/security/oauth/` y la migración `V4__initial_account_and_bonus_invariants.sql`. **Todo el análisis se refiere a ese estado sin commitear.**

## Convenciones

**Etiquetas**

| Etiqueta | Significado |
|---|---|
| **[VERIFICADO]** | Leído en el código o la configuración citados. |
| **[INFERIDO (motivo)]** | Deducción razonada; no se ha ejecutado. |
| **[NO ENCONTRADO]** | Se buscó y no existe. |

**Rutas**

Son relativas a `imaginary_bank/`, salvo las que empiezan por `bank-contracts/` o `bank-bff-openapi/`, que son repositorios hermanos. Para abreviar se usan estos alias:

| Alias | Ruta |
|---|---|
| `ACC/` | `micro/account/src/main/java/com/tagokoder/account/` |
| `IDN/` | `micro/identity/src/main/java/com/tagokoder/identity/` |
| `LED/` | `micro/ledger/internal/` |
| `BFF/` | `bff/internal/` |
| `WEB/` | `imaginarybank-web/src/app/` |

## Resumen ejecutivo

Imaginary Bank es una banca digital académica formada por:
- una SPA Angular;
- un BFF en Go, que gestiona la sesión con cookie y hace de relay de tokens;
- tres microservicios gRPC: **identity** (Java), **account** (Java) y **ledger/payments** (Go).

El inicio de sesión se hace con Cognito mediante OIDC, gestionado por identity. La infraestructura AWS está descrita en CloudFormation (ECS Fargate, RDS, ElastiCache, AVP, EventBridge y SQS).

La arquitectura **pretende** ser Zero Trust y usar autorización centralizada en Cedar/AVP, pero el código actual **no la aplica de forma efectiva**:

1. **Account:** el DENY de AVP solo se registra en el log y la petición continúa (`ACC/infra/security/grpc/AuthzServerInterceptor.java:269-284`).
2. **Identity:** ninguna ruta pasa por AVP (`IDN/infra/security/authz/RouteAuthzRegistry.java:16-40`).
3. **Ledger:** envía a AVP acciones que no existen en el esquema Cedar (`LED/infra/security/avp/client.go:49`). Con el esquema actual, AVP lo denegaría todo.
4. **Políticas Cedar:** no comprueban MFA, umbral de importe ni estado de la cuenta (`infra/authz/cedar/policies/*.cedar`).
5. **Cognito:** no emite `custom:customer_id`, `amr` ni scopes de servicio.
6. **AVP:** el policy store no tiene identity source (`infra/cloudformation/70-authz-avp/template.yml:20-32`).

Hay dos comprobaciones de titularidad que sí son reales: `ListAccounts` usa el cliente del token y `ActivateRegistration` comprueba que el registro pertenece al usuario.

El despliegue en AWS, tal como está escrito, no llegaría a funcionar (ver [01](01-arquitectura-actual.md)). Hay **contraseñas maestras de RDS en claro versionadas en git** (`infra/params/{dev,prod}/20-data.json:13`).

Conclusión para el TFE: **no existe todavía una línea base medible ejecutable de punta a punta**. Los flujos de pago y de movimientos fallarían antes de llegar a la decisión de autorización. Sí existe una línea base **analítica**, documentada en [06](06-escenarios-TFE.md).

## Repositorios y componentes

| Componente | Ruta | Tecnología | Versiones clave | Evidencia |
|---|---|---|---|---|
| Frontend web | `imaginarybank-web/` | Angular + PrimeNG | Angular ^20.3.16, PrimeNG ^20.4, TS 5.8 | `imaginarybank-web/package.json:13-40` [VERIFICADO] |
| BFF | `bff/` | Go, chi, oapi-codegen strict server | Go 1.24.4, chi v5.2.3, grpc v1.78.0 | `bff/go.mod:3-14` [VERIFICADO] |
| identity | `micro/identity/` | Spring Boot (gRPC net.devh) | Boot 4.0.0, Java 25, gRPC 1.66.0, AWS SDK 2.25.x | `micro/identity/pom.xml:8-31` [VERIFICADO] |
| account | `micro/account/` | Spring Boot (gRPC net.devh) | Boot 4.0.0, Java 25, gRPC 1.66.0, AWS SDK 2.41.9 | `micro/account/pom.xml:7-53` [VERIFICADO] |
| ledger/payments | `micro/ledger/` | Go, gRPC, GORM, golang-jwt | go 1.26.5 en go.mod (imagen golang 1.24.4), grpc v1.84.0 | `micro/ledger/go.mod:3-19`, `micro/ledger/Dockerfile:6` [VERIFICADO] |
| Contratos gRPC | `../bank-contracts` | buf v2, protoc | tags v0.1.0–v0.3.0; Java `bank-contracts-java:0.3.0`; Go `github.com/tagoKoder/bank-contracts` | `bank-contracts/pom.xml`, `bank-contracts/go.mod` [VERIFICADO] |
| Contrato REST del BFF | `../bank-bff-openapi` | OpenAPI 3.0.3 publicado como paquete npm | `@tagokoder/bff-openapi` v0.0.20 | `bank-bff-openapi/openapi/bff.openapi.yaml:1`, `bff/package.json:5` [VERIFICADO] |
| Infraestructura | `infra/` | CloudFormation (9 stacks), Cedar, PowerShell | — | `infra/cloudformation/*` [VERIFICADO] |
| Entorno local | `docker-compose.yml` | Postgres 16, Redis 7, LocalStack 3.5, Flyway 10 | — | `docker-compose.yml:18-207` [VERIFICADO] |

> El BFF **no** consume `bank-contracts`: exporta del Buf Schema Registry `buf.build/imaginarybank/bank:v0.0.19` y genera su propio código (`bff/contracts/proto.ref.txt:1`). Account, identity y ledger sí usan `bank-contracts` v0.3.0. [VERIFICADO]

## Cómo se ejecuta (local)

| Servicio | Puerto host:contenedor | Notas |
|---|---|---|
| postgres | 5442:5432 | Una base de datos por servicio: `infra/local/postgres/init/01-create-databases.sql:3-6` |
| redis | 6379:6379 | Sin contraseña |
| localstack | 4566:4566 | Monta `docker.sock` |
| ledger-migrate | — | Flyway |
| identity | 19091:9091 | |
| account | 19092:9092 | |
| ledger | 19093:9093 | Por defecto el código escucha en 8082 si no se define `GRPC_PORT`: `micro/ledger/internal/infra/config/config.go:67` |
| bff | (8080) | **Comentado** en `docker-compose.yml:185-207` |

- El frontend se arranca aparte con `ng serve` y un proxy de `/api` y `/bff` a `localhost:8080` (`imaginarybank-web/proxy.conf.json:2-13`). [VERIFICADO]
- `docker compose up` **no funciona tal cual**: faltan `micro/*/.env`, `infra/local/localstack/init` y `./.aws`. [VERIFICADO: ficheros ausentes]
- Los perfiles de seguridad dependen de esos `.env`, que no existen, así que **no se puede saber con qué perfil se ejecutaba**:
  - identity usa `local` por defecto (`micro/identity/src/main/resources/application.yml:5`), que significa sin autenticación ni autorización.
  - account no fija perfil, de modo que no es `local` y la autorización está activa (`micro/account/src/main/resources/application.yml:1-12`).
  - ledger tiene la autorización siempre activa. [VERIFICADO / INFERIDO]

**Variables de entorno principales** (solo nombres; no se copian valores):

| Componente | Variables |
|---|---|
| identity | `SPRING_PROFILES_ACTIVE`, `SPRING_DATASOURCE_*`, `REDIS_*`, `COGNITO_ISSUER`, `COGNITO_CLIENT_ID`, `COGNITO_CLIENT_SECRET`, `AVP_POLICY_STORE_ID`, `IDENTITY_SERVICE_ACCESS_TOKEN`, `IDENTITY_*_ENC_KEY_B64`, `IDENTITY_REFRESH_TOKEN_PEPPER`, `KYC_*`, `ACCOUNTS_TARGET` |
| account | `COGNITO_ISSUER_URI`, `COGNITO_AUDIENCE`, `AVP_POLICY_STORE_ID`, `COGNITO_TOKEN_URL`, `COGNITO_CLIENT_ID`, `COGNITO_CLIENT_SECRET`, `COGNITO_SERVICE_SCOPE`, `LEDGER_GRPC_TARGET`, `IDENTITY_GRPC_TARGET`, `LEDGER_EVENTS_QUEUE_URL`, `FLYWAY_ENABLED` |
| ledger | `APP_ENV`, `GRPC_PORT`, `JWT_ISSUER`, `JWT_AUDIENCE`, `JWT_JWKS_URL`, `AVP_POLICY_STORE_ID`, `DB_*DSN`, `ACCOUNTS_GRPC_*`, `ACCOUNTS_INTERNAL_TOKEN`, `LEDGER_SYSTEM_CREDIT_CLIENT_IDS`, `LEDGER_SYSTEM_CREDIT_SCOPE`, `DOMAIN_EVENTBUS_NAME`, `AUDIT_EVENTBUS_NAME` |
| BFF | `HTTP_ADDR`, `COOKIE_SECURE`, `COOKIE_SAMESITE`, `REDIRECT_ALLOWLIST`, `RATE_LIMIT_RPS`, `IDENTITY_GRPC_ADDR`, `ACCOUNT_GRPC_ADDR`, `LEDGER_GRPC_ADDR`, `GRPC_USE_TLS`, `GRPC_CA_CERT`, `GRPC_CLIENT_CERT`, `GRPC_CLIENT_KEY`, `REDIS_*` |

Evidencia: `micro/identity/src/main/resources/application.yml`, `micro/account/src/main/resources/application.yml:14-98`, `micro/ledger/internal/infra/config/config.go:64-106`, `bff/internal/config/config.go:41-71`. [VERIFICADO]

**AWS:** se despliega con `infra/cloudformation/scripts/02-deploy-all-dev.ps1`, en el orden 00 → 10 → 20 → 30 → 70 (y aplicación de políticas AVP) → 80 → 60 → 40 → 50. El script de producción `03-deploy-all-prod.ps1` está roto. Detalle en [01](01-arquitectura-actual.md).

## Documentos

| # | Documento | Contenido |
|---|---|---|
| 01 | [01-arquitectura-actual.md](01-arquitectura-actual.md) | Componentes, llamadas, bases de datos, despliegue local frente a AWS y diagrama con fronteras de confianza |
| 02 | [02-contratos-y-endpoints.md](02-contratos-y-endpoints.md) | Todos los endpoints REST y gRPC, con autorización, evidencia y diferencias entre contrato e implementación |
| 03 | [03-flujos.md](03-flujos.md) | Secuencias de login/registro, apertura de cuenta, cuentas, saldo, movimientos y transferencia |
| 04 | [04-identidad-y-autorizacion.md](04-identidad-y-autorizacion.md) | Cognito, JWT, claims, propagación, autenticación entre servicios, Cedar/AVP y fallo seguro |
| 05 | [05-modelo-de-datos.md](05-modelo-de-datos.md) | Tablas, titularidad, estados, importes, límites y umbrales |
| 06 | [06-escenarios-TFE.md](06-escenarios-TFE.md) | Escenarios E1–E12 con su resultado actual y lo que falta; conclusión sobre la línea base |
| 07 | [07-pendientes-y-objetivo.md](07-pendientes-y-objetivo.md) | TODO, inconsistencias, tests, README frente a código y hoja de ruta priorizada con esfuerzo S/M/L |
