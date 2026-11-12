# 07 · Pendientes, inconsistencias y camino hacia la arquitectura objetivo

Convenciones y alias: ver [00-INDICE](00-INDICE.md).

## 1. TODO/FIXME y marcadores de trabajo pendiente

No hay marcadores literales `TODO`/`FIXME` en el código propio de ningún componente. [NO ENCONTRADO] Sí hay comentarios equivalentes [VERIFICADO]:

| Ubicación | Comentario |
|---|---|
| `ACC/infra/out/grpc/LedgerGrpcClientAdapter.java:45` | "Fase 3: agregar credenciales de servicio verificadas y autorización" |
| `ACC/application/service/AccountOpeningService.java:127` | "Fase 4", sobre el saldo por eventos |
| `micro/account/src/main/resources/db/migration/V4__initial_account_and_bonus_invariants.sql:1` | "FASE 1" |
| `ACC/application/service/CustomerService.java:39-43` | La dirección no se persiste |
| `ACC/infra/in/grpc/validation/InternalAccountsGrpcValidators.java:117` | "aunque HOY ignores holdId/idem" |
| `ACC/infra/security/authz/AccountResourceTemplates.java:25` | "lo vamos a agregar en schema" |
| `IDN/application/service/OidcAuthService.java:226` | "crear este método" |
| `IDN/infra/out/grpc/AccountsGrpcClientAdapter.java:10` | "Ajusta imports … según tu proto real" |
| `LED/infra/in/grpc/validation/ledger_system.go:53` | "MVP: solo permitimos registration_bonus" |
| `LED/application/service/ledger_service.go:492` | "en demo: puedes decidir si fallar o degradar" |
| `infra/cloudformation/00-foundation/template.yml:433` | "Tighten later", sobre AdministratorAccess |
| `infra/cloudformation/50-compute-ecs/template.yml:180` | "expand in Entregable 7 for AVP/EventBridge/S3 KYC" |
| `infra/params/*/50-compute-ecs.json:10`, `infra/params/prod/60-audit-observability.json:18-29` | `REEMPLAZA_TAG`, `REEMPLAZAR_*` |

## 2. Código incompleto o inconsistente (principales)

### Seguridad
- **Contraseña maestra de RDS en claro, versionada en git**: `infra/params/dev/20-data.json:13` e `infra/params/prod/20-data.json:13`. **Hay que rotarla y purgarla del historial.** [VERIFICADO]
- **Account:** DENY de AVP no aplicado y auditoría que registra `ALLOW` (`ACC/infra/security/grpc/AuthzServerInterceptor.java:269-284`). [VERIFICADO]
- **Account:** rutas internas como `PUBLIC` (`CreateCustomer`, `CreateAccount`, `OpenAccountWithOpeningBonus`). Rutas sin registrar (`GetAccountByNumber`, `BatchGetAccountSummaries`), que provocan un NPE. [VERIFICADO]
- **Identity:**
  - Perfil `local` por defecto, sin autorización (`micro/identity/src/main/resources/application.yml:5`).
  - Ninguna ruta en modo AUTHZ.
  - `GetSessionInfo` devuelve el access token a cualquiera que presente el `session_id`.
  - Un `RefreshSession` resucita sesiones caducadas por inactividad (`IDN/application/service/SessionService.java:132-134`).
  - Reactiva usuarios en estado LOCKED (`IDN/application/service/OidcAuthService.java:151`). [VERIFICADO]
- **Ledger:**
  - Acciones fuera del esquema Cedar (`LED/infra/security/avp/client.go:49`).
  - `iss`/`aud` opcionales.
  - `CreditAccount` accesible con token de usuario.
  - Idempotencia global.
  - Reflection activa fuera de local (`micro/ledger/cmd/ledger/main.go:148`).
  - El DSN se escribe en el log (`main.go:51`). [VERIFICADO]
- **BFF:**
  - Escribe en el log el `session_id` y la cabecera Cookie (`BFF/api/rest/middleware/auth_session.go:45,66-74`).
  - Toda denegación se convierte en 502.
  - IP falsificable con `RealIP`.
  - CSRF no rotado en el login.
  - `user_status` vacío deja pasar (fail-open). [VERIFICADO]
- **Infraestructura:**
  - Grupos de seguridad sin reglas de entrada.
  - Sin WAF en el ALB.
  - Service Connect y Redis sin TLS.
  - Un único usuario de BD compartido.
  - Rol de despliegue con `AdministratorAccess`. [VERIFICADO]

### Funcional
- **Plantilla `ValidateAccountsAndLimits`** en account: llama a `getId()`, que no existe (`ACC/infra/security/authz/AccountResourceTemplates.java:65-67`).
- **Interceptor de idempotencia** en account: no está registrado, así que `PatchAccountLimits` falla (`ACC/infra/in/grpc/AccountsGrpcService.java:163`).
- **`ServiceTokenProvider`** en account: lee propiedades con un prefijo que no coincide con el del yml (`ACC/infra/security/oauth/ServiceTokenProvider.java:24-34`).
- **Holds:** la tabla `account_holds` nunca se escribe y `hold_id` se ignora.
- **`GetPayment`** en ledger: bug de tipos `*uuid.UUID` (`LED/infra/security/authz/resource_resolver_reflect.go:47`).
- **Paginación** de `ListAccountJournalEntries` en ledger: base 0 frente a base 1.
- **`ActivateRegistration`:** identity exige JWT (`AUTHN_ONLY`), pero el BFF no envía bearer en esa ruta pública.
- **Panic en el BFF** si `accepted_terms` llega vacío (`BFF/client/grpc/identity.go:241`).
- **Dial gRPC del BFF:** bloquea sin timeout al arrancar (`BFF/client/grpc/conn.go:23-28`).
- **Access log del BFF:** siempre registra 200 (`BFF/api/rest/middleware/access_log.go:25-33`).
- **Puertos incoherentes:** ledger 8082/9093/7003, identity y account 9091/9092/7001/7002, destinos por defecto `:9090`.
- **Variables de entorno de ECS** que el código no lee, y parámetros obligatorios ausentes en `params/*/50-compute-ecs.json`.
- **Script de despliegue de prod** roto: `infra/cloudformation/scripts/03-deploy-all-prod.ps1:4,15`.
- **Toolchain de ledger:** `go.mod` exige Go 1.26.5, pero la imagen Docker usa 1.24.4.
- **Contratos:**
  - El BFF genera desde BSR v0.0.19 en lugar de usar `bank-contracts` v0.3.0.
  - Vocabularios de estado incoherentes en el proto.
  - El `go_package` de ops es incorrecto.
- **Frontend:**
  - `inject()` fuera del contexto de inyección en el reintento de CSRF (`WEB/core/interceptors/csrf.interceptor.ts:46`).
  - La `Idempotency-Key` se regenera en cada clic (`WEB/features/home/home.ts:217-220`).
  - Servicios `admin` y `webauthn` obsoletos.

## 3. Tests existentes

| Componente | Tests | Qué cubren | Evidencia |
|---|---|---|---|
| identity | 1 (`contextLoads`, con Testcontainers) | Solo el arranque; probablemente falla sin Redis, AWS ni claves | `micro/identity/src/test/java/com/tagokoder/identity/IdentityApplicationTests.java:7-14` [VERIFICADO] |
| account | 1 (`contextLoads`) | Solo el arranque | `micro/account/src/test/java/com/tagokoder/account/AccountApplicationTests.java:7-15` [VERIFICADO] |
| ledger | **0** | — | `find micro/ledger -name "*_test.go"` [VERIFICADO] |
| BFF | **0** | — | [VERIFICADO] |
| web | 4 specs de plantilla, probablemente rotas (`app.spec.ts` espera un `<h1>` que no existe; no hay `HttpClient` en los providers) | Nada funcional | `WEB/app.spec.ts:21` [VERIFICADO / INFERIDO] |
| Cedar | **0** (no hay tests de políticas ni `cedar validate` en CI) | — | [NO ENCONTRADO] |
| CI | `bank-contracts`: buf lint y breaking. `bank-bff-openapi`: solo publicación, sin el lint de Spectral. | — | `.github/workflows/*` [VERIFICADO] |

## 4. Documentación desactualizada (README frente a código)

| Afirmación | Realidad | Evidencia |
|---|---|---|
| "Centralized authorization with AWS Verified Permissions", "Deny-by-default" | Account no aplica el DENY, identity no usa AVP y las acciones de ledger no están en el esquema | `README.md:32-33` frente a [04](04-identidad-y-autorizacion.md) [VERIFICADO] |
| "AVP is queried to decide ALLOW or DENY" (flujo, paso 6) | Falso en identity (todas sus rutas son PUBLIC o AUTHN_ONLY) | `README.md:201` [VERIFICADO] |
| Zero Trust, defensa en profundidad, OWASP ASVS L3 | gRPC sin TLS, sin identidad de servicio, BD compartida. `common.json` (asvs-l3) no lo usa nadie. | `README.md:7` [VERIFICADO] |
| `docker compose up -d` | Falla: faltan `.env`, el init de LocalStack y `.aws`; el BFF está comentado | `README.md:244-245` [VERIFICADO] |
| La estructura incluye `.aws/`, `.vscode/` y `arch.md` | No existen; `arch.md` está borrado en el árbol de trabajo | `README.md:82-92` [VERIFICADO] |
| READMEs de los stacks 10, 50 y 70 | SG "cableados en 50" (no lo están), "listener 80" (no existe), rutas de scripts incorrectas | `infra/cloudformation/10-network/README.md:8`, `50-compute-ecs/README.md:1-8`, `70-authz-avp/README.md:4-7` [VERIFICADO] |
| README de la web | Plantilla del Angular CLI: no menciona el proxy, el BFF ni `gen:bff` | `imaginarybank-web/README.md` [VERIFICADO] |
| README de `bank-contracts` y `bank-bff-openapi` | Vacíos o solo con el título | [VERIFICADO] |

## 5. Hoja de ruta priorizada hacia la arquitectura objetivo

**Objetivo:** autorización en cada servicio de dominio y entre servicios, con políticas Cedar que combinen titularidad, contexto y riesgo (importe frente a umbral, antigüedad y método de autenticación, estado de la cuenta), y una decisión de permitir o denegar con un **motivo trazable**.

Esfuerzo: **S** = menos de 1 día · **M** = 1–3 días · **L** = más de 3 días.

### P0: higiene e infraestructura habilitante (necesario para la línea base 0)

| # | Tarea | Esfuerzo | Desbloquea |
|---|---|---|---|
| 0.1 | Rotar la contraseña de RDS, sacar `DbMasterPassword` del repo (Secrets Manager con `ManageMasterUserPassword`) y purgar el historial | S | Seguridad |
| 0.2 | Entorno local reproducible: `.env.example` por servicio, BFF activo en compose, puertos y destinos unificados, `SPRING_PROFILES_ACTIVE` explícito, init de LocalStack | M | Todo |
| 0.3 | Corregir los bugs funcionales: plantilla `ValidateAccountsAndLimits` (`source_account_id`), interceptor de idempotencia en account, prefijo de `ServiceTokenProvider`, registrar `GetAccountByNumber` y `BatchGetAccountSummaries`, bug de tipos de `GetPayment`, bearer en `ActivateRegistration` | M | E3, E5, E8 |
| 0.4 | BFF: usar `httperr.MapGrpc` (401, 403, 404, 409 y 503, más un código `STEP_UP_REQUIRED`) e incluir `reason` en `ErrorResponse` (OpenAPI) | S | Medición de todos los escenarios |
| 0.5 | Arnés de escenarios E1–E12 automatizado (script con `grpcurl`/`curl` o Newman) con datos semilla de dos clientes, una cuenta `frozen` y tokens con y sin MFA | M | Línea base medible |

### P1: modelo de autorización Cedar (núcleo del TFE)

| # | Tarea | Esfuerzo |
|---|---|---|
| 1.1 | Rediseñar `schema.json`: tipos `Customer` (principal) y `Service`; `Account{owner, status}`; contexto `{amount: decimal, currency, auth_time_age_s, mfa, channel, risk_level}`; acciones unificadas (`account:read`, `account:read_balance`, `statement:read`, `payment:create`, `payment:read`, `ledger:credit_system`, `account:hold_reserve`…). Usar la extensión `decimal` para los importes. | M |
| 1.2 | Políticas: `permit` por titularidad (`principal == resource.owner`); `forbid` con `@id("deny_inactive_account")` para el estado; `forbid` con `@id("deny_high_amount_without_recent_mfa")` cuando `amount > threshold && (!mfa \|\| auth_age > N)`; `permit` de servicio por acción. **El motivo trazable sale del `@id` de la política determinante.** | M |
| 1.3 | Origen del umbral: atributo de entidad (`Account.threshold` o `Customer.risk_segment`), un parámetro del policy store o un *policy template*. Hay que **impedir que el cliente modifique `risk_segment`** (`PatchCustomer`). | S–M |
| 1.4 | Tests de políticas con `cedar validate` y `cedar authorize` (CLI) más casos E1–E12 en CI | M |
| 1.5 | AVP: crear un `IdentitySource` en el stack 70 o cambiar a `IsAuthorized` enviando entidades explícitas (recomendado: control total de los atributos y sin depender de los claims de Cognito); `ValidationSettings: STRICT`; una política por fichero | M |

### P2: PEP en cada servicio de dominio

| # | Tarea | Esfuerzo |
|---|---|---|
| 2.1 | Account: **aplicar el DENY** (`AuthzServerInterceptor.java:269`) y reflejar el resultado real en la auditoría; comprobación defensiva de titularidad en el servicio (`enforceSelfOnlyCustomer`) | S |
| 2.2 | Ledger: resolver el propietario (`source_account_owner_customer_id`) con account, o recibir el resumen de la cuenta, y enviar la entidad `Payment` con importe y estado a AVP; acciones con namespace | M |
| 2.3 | Ledger y account: construir el contexto de riesgo con `amount`, `auth_time` (del JWT), `mfa` y `channel`, a partir de fuentes de confianza y no de cabeceras del cliente | M |
| 2.4 | Identity: mover `GetSessionInfo`, `RefreshSession` y `LogoutSession` a modo servicio (solo el BFF, con mTLS o M2M); perfil seguro por defecto; no reactivar usuarios LOCKED; comprobar inactividad en el refresh | M |
| 2.5 | Propagar el motivo: `PERMISSION_DENIED` con `ErrorInfo` (reason = `@id`) → BFF → HTTP 403 o 401 `STEP_UP_REQUIRED` | S–M |

### P3: identidad de servicio (Zero Trust entre servicios)

| # | Tarea | Esfuerzo |
|---|---|---|
| 3.1 | Cognito: resource server con scopes por operación (`ledger/credit_system`, `accounts/hold`, `accounts/validate`…) y un client M2M por servicio | M |
| 3.2 | Rutas internas de account en modo `SERVICE_ONLY` (scope y `client_id`); el BFF deja de llamar a `ValidateAccountsAndLimits` | M |
| 3.3 | Account debe aceptar varios `client_id` (hoy solo acepta el client de la web: `ACC/infra/config/JwtConfig.java:38-62`); `JwtConfig` por tipo de ruta | S |
| 3.4 | Sustituir el JWT estático de identity por client_credentials; eliminar `x-internal-token` | S |
| 3.5 | mTLS entre servicios (Service Connect TLS con ACM PCA) o, como mínimo, TLS y reglas de SG mínimas | L |

### P4: autenticación reciente y step-up

| # | Tarea | Esfuerzo |
|---|---|---|
| 4.1 | Señal de MFA o autenticación reciente: leer `auth_time` del access token; para MFA, un trigger pre-token V2 que añada un claim `mfa` o `amr`, o guardar en la sesión de identity el método de autenticación | M |
| 4.2 | Flujo de step-up: el BFF recibe `STEP_UP_REQUIRED`, la SPA redirige a `/auth/oidc/start?max_age=0&prompt=login` y, al volver, se reintenta con la misma `Idempotency-Key` | M–L |
| 4.3 | MFA obligatorio en dev o en usuarios de prueba para poder medir E5 frente a E7 | S |

### P5: observabilidad y fallo seguro

| # | Tarea | Esfuerzo |
|---|---|---|
| 5.1 | Timeouts y circuit breaker hacia AVP; métrica y alarma de `avp_error`, `deny_rate` y latencia | S–M |
| 5.2 | Auditoría de decisiones con `determiningPolicies`, `reason`, principal, recurso y contexto saneado; consumidor del audit-bus (hoy solo se archiva) | M |
| 5.3 | Eliminar los logs de `session_id`, Cookie y DSN; desactivar la reflection en prod | S |

### P6: deuda de despliegue (solo si el TFE requiere una demostración en AWS)

| # | Tarea | Esfuerzo |
|---|---|---|
| 6.1 | Stack 50: variables y puertos coherentes, `GetSecretValue`, reglas de SG, WAF en el ALB, parámetros completos | L |
| 6.2 | Una BD o un rol por servicio; Redis con TLS y autenticación | M |
| 6.3 | Corregir `03-deploy-all-prod.ps1` y los parámetros de prod de Cognito | S |

**Camino mínimo recomendado para el TFE:** 0.2 → 0.3 → 0.4 → 0.5 (**medir la línea base 0**) → 1.1 → 1.2 → 1.4 → 1.5 → 2.1 → 2.2 → 2.3 → 2.5 → 4.1 → 3.1/3.2 (**medir la línea base 1**). El resto es mejora. Esfuerzo total estimado del camino mínimo: **~4–6 semanas** para una persona. [INFERIDO]
