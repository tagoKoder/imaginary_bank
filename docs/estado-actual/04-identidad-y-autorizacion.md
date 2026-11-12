# 04 · Identidad y autorización

Convenciones y alias: ver [00-INDICE](00-INDICE.md).

## 1. Cognito (configuración en IaC)

`infra/cloudformation/30-identity-cognito/template.yml` [VERIFICADO salvo indicación]

| Aspecto | Valor | Evidencia |
|---|---|---|
| Login | Por email, con verificación automática del email | `:53,70-73` |
| Contraseña | 12 caracteres, todos los tipos | `:56-63` |
| MFA | `SOFTWARE_TOKEN_MFA` (TOTP). Por defecto `OPTIONAL`; dev `OPTIONAL`, prod `ON`. No hay SMS ni WebAuthn. | `:31-34,65-67`; `infra/params/dev/30-identity-cognito.json:7`; `infra/params/prod/30-identity-cognito.json:8` |
| Advanced security | `ENFORCED` (requiere el plan Plus; no se fija `UserPoolTier`) | `:76` [INFERIDO sobre el plan] |
| App client | Uno solo, `bff`, **público** (`GenerateSecret: false`). Flujo `code`, scopes `openid email profile`. | `:78-104` |
| Validez de tokens | access 60 min, id 60 min, refresh 30 días | `:37-45,98-104` |
| Resource server / scopes propios | **No existen** | [NO ENCONTRADO] |
| Client credentials (M2M) | **No existe ningún client** | [NO ENCONTRADO] |
| Atributo `custom:customer_id` | **No declarado** | [NO ENCONTRADO] |
| Grupos | **No existen** | [NO ENCONTRADO] |
| Lambda pre-token generation | **No existe** | [NO ENCONTRADO] |
| Callbacks en prod | Los params usan `CallbackUrl`/`LogoutUrl` en singular, pero la plantilla espera plurales, así que el despliegue falla | `infra/params/prod/30-identity-cognito.json:5-7` |

**Conflictos entre el código y Cognito:**
- El código espera `COGNITO_CLIENT_SECRET` (identity y account) y `COGNITO_SERVICE_SCOPE` / `LEDGER_SYSTEM_CREDIT_SCOPE` (account y ledger), pero en IaC no hay client con secreto ni scopes propios. [VERIFICADO]
- Los access tokens de Cognito **normalmente no incluyen** `amr` ni atributos `custom:*` si no hay un trigger pre-token V2. En consecuencia, `mfa_verified` vale siempre `false` y `custom:customer_id` falta; el `customer_id` se resuelve llamando a identity con `ResolvePrincipal`. [INFERIDO: comportamiento documentado de Cognito, **no verificado con un token real**]
- `auth_time` existe en los tokens de Cognito, pero **ningún componente lo lee**. [NO ENCONTRADO en BFF, identity, account y ledger]

## 2. Dónde y cómo se valida el JWT

| Componente | Valida | Detalle | Evidencia |
|---|---|---|---|
| BFF | **Nada** | Trata el token como opaco y confía en lo que devuelve identity `GetSessionInfo`. El navegador solo tiene la cookie opaca. | `BFF/api/rest/middleware/auth_session.go:108-216` [VERIFICADO] |
| identity (perfil distinto de `local`) | Access token | `fromIssuerLocation` (JWKS), `iss`, `exp`/`nbf`, `token_use=access`, `client_id` o `aud` igual a `COGNITO_CLIENT_ID` | `IDN/infra/config/JwtDecoderConfig.java:21-67` [VERIFICADO] |
| identity | ID token (en el login) | Firma, `iss`, `aud`, `token_use=id`, `nonce` | `IDN/infra/security/OidcIdTokenValidator.java:29-60` [VERIFICADO] |
| identity (perfil `local`, **el perfil por defecto**) | **Nada** | `NoAuthzServerInterceptor` | `micro/identity/src/main/resources/application.yml:5`, `IDN/infra/config/SecurityWiringConfig.java:30-34` [VERIFICADO] |
| account (perfil distinto de `local`, que es el comportamiento por defecto) | Access token | JWKS `${issuer}/.well-known/jwks.json`, `iss`, `exp`/`nbf`, `token_use=access`, `client_id` o `aud` igual a `COGNITO_AUDIENCE`. **Esto rechaza tokens de otros clients, incluidos los M2M.** | `ACC/infra/config/JwtConfig.java:33-62` [VERIFICADO / INFERIDO M2M] |
| ledger | Access o ID token | Solo RS256 con `kid` obligatorio, `exp`, `sub`. **`iss` y `aud` solo si están configurados** (ECS no los configura). `token_use` solo se comprueba en `SERVICE_ONLY`. JWKS en caché 10 min, refresco sin límite. | `LED/infra/security/jwt/validator.go:82-197,339-385` [VERIFICADO] |

**Claims usados:**

| Claim | identity | account | ledger | BFF |
|---|---|---|---|---|
| `sub` | Principal `poolId\|sub` | Principal `poolId\|sub` | Obligatorio y actor | — (`subject_id_oidc` de identity) |
| `iss` | Sí | Sí | Solo si está configurado | — |
| `token_use` | `access` | `access` | Solo en SERVICE_ONLY | — |
| `client_id` / `aud` | Sí | Sí | Lista blanca en SERVICE_ONLY | — |
| `scope` | No | No | SERVICE_ONLY | — |
| `cognito:groups` | Roles (no se usan) | Roles (no se usan) | Roles (no se usan) | Roles guardados y no usados |
| `amr` | Pasa a `mfa_verified` (contexto AVP, sin rutas AUTHZ) | Pasa a `mfa_verified` en el contexto AVP | Pasa a `MFA`, **solo para auditoría** | — |
| `custom:customer_id` | Se lee | Se lee y se sobrescribe con `ResolvePrincipal` | Se lee | — |
| `auth_time` | **No** | **No** | **No** | **No** |

Evidencias: `IDN/infra/security/grpc/AuthzServerInterceptor.java:119-123`, `ACC/infra/security/grpc/AuthzServerInterceptor.java:122-154,221`, `LED/infra/security/jwt/validator.go:128-150` y `LED/infra/out/audit/eventbridge_audit.go:55`. [VERIFICADO]

## 3. Propagación de la identidad entre servicios

- **BFF → micros: token relay.** El BFF envía `authorization: Bearer <access token del usuario>` junto con `x-correlation-id`, `x-route-template`, `x-forwarded-for`, `x-user-agent` e `idempotency-key` (`BFF/client/grpc/outgoing_metadata.go:35-64`). No envía `x-customer-id` (`BFF/client/grpc/meta.go` no se usa). [VERIFICADO]
- **Desajuste de cabeceras.** identity espera `x-ip-hash`, `x-ua-hash` y `x-idempotency-key` (`IDN/infra/security/grpc/CorrelationServerInterceptor.java:11-24`). Esas señales llegan vacías. [INFERIDO]
- **Resolución del cliente.** account llama a identity `ResolvePrincipal` con el bearer del usuario para obtener el `customer_id` (`ACC/infra/security/grpc/AuthzServerInterceptor.java:127-148`). Si identity falla, el error se ignora y solo se deniega si falta el `customer_id`. [VERIFICADO]
- **ledger → account.** Reenvía el bearer del usuario (`LED/infra/out/gateway/accounts_http_gateway.go:92-118`). [VERIFICADO]
- **Contexto de riesgo.** IP y UA se calculan a partir de cabeceras que controla el cliente: `chimw.RealIP` en el BFF (`BFF/api/rest/server/router.go:39`). Son falsificables. [INFERIDO]

## 4. Autenticación entre servicios

| Llamada | Mecanismo | Evaluación | Evidencia |
|---|---|---|---|
| BFF → identity | Ninguno: red y mTLS opcional | El `session_id` es la única credencial, y `GetSessionInfo` devuelve tokens a quien lo presente | `BFF/client/grpc/conn.go:31-66` [VERIFICADO] |
| identity → account | JWT estático `IDENTITY_SERVICE_ACCESS_TOKEN`, sin renovación | Caduca en 60 min. Account lo recibe en rutas PUBLIC, así que ni siquiera lo valida. | `IDN/infra/config/GrpcClientsConfig.java:19-31` [VERIFICADO] |
| account → identity | Bearer del usuario | `ResolvePrincipal` acepta cualquier token del pool | `IDN/infra/in/grpc/PrincipalGrpcService.java:45-57` [VERIFICADO] |
| account → ledger | client_credentials de Cognito (`ServiceTokenProvider`) | Es el único M2M "real". Ledger verifica la lista blanca de `client_id` y el `scope`. Sin embargo, el client no existe en IaC y las propiedades están mal nombradas. | `ACC/infra/security/oauth/ServiceTokenProvider.java:24-106`, `LED/infra/security/jwt/validator.go:339-385` [VERIFICADO] |
| ledger → account | Bearer del usuario + `x-internal-token` estático | Account **no verifica** `x-internal-token` | `LED/infra/out/gateway/accounts_http_gateway.go:110-113`; grep `internal-token` en account sin resultados [VERIFICADO / NO ENCONTRADO] |
| Transporte | gRPC en claro en todos los tramos; Service Connect sin TLS | Sin mTLS | `ACC/infra/config/GrpcClientsConfig.java` (`usePlaintext`), `micro/ledger/internal/infra/config/config.go:99` [VERIFICADO] |

## 5. Cedar / Amazon Verified Permissions: estado real

### 5.1 Artefactos
- **Esquema:** `infra/authz/cedar/schema.json`, namespace `ImaginaryBank`. [VERIFICADO]
  - Entidades: `User{customer_id, roles}`, `Account{account_id, owner_customer_id, status, owner}`, `Customer{customer_id, status, owner}` y `Payment{payment_id, owner_customer_id, source_account_id, source_account_owner_customer_id, dest_account_id, amount: Long, currency}`.
  - Hay 11 acciones, todas con principal `User`. Contexto declarado: `accounts:read{customer_id}` y `payments:create{mfa_verified}`.
  - **No hay tipo Service**, ni `auth_time`, ni umbral, ni `risk`.
- **Políticas** (`infra/authz/cedar/policies/`), todas `permit` y **ninguna `forbid`** [VERIFICADO]:

  | Fichero | Acción | Condición | Problema |
  |---|---|---|---|
  | `01_accounts_read_own.cedar` | `accounts:read` | `context.customer_id == resource.owner_customer_id` | La titularidad se compara con un valor que **envía el llamante**, no con el principal. |
  | `02_payments_create_mfa.cedar` | `payments:create` | `principal.customer_id == resource.source_account_owner_customer_id` | **No comprueba `mfa_verified`** a pesar de su nombre. |
  | `03_account_balances.cedar` | `balances_read`, `limits_patch` | `principal.customer_id == resource.owner_customer_id` | Hay dos sentencias en un único fichero, y el script crea una política por fichero; AVP probablemente lo rechaza. [INFERIDO] |

- **Acciones sin ninguna política** (deny por defecto): 7 de 11. Son `accounts:create`, `accounts:validate`, `accounts:hold_reserve`, `accounts:hold_release`, `customers:create`, `customers:patch` y `payments:read`. [VERIFICADO]
- **Policy store** (stack 70): `ValidationSettings OFF` y **sin `IdentitySource`** (`infra/cloudformation/70-authz-avp/template.yml:20-32`). [VERIFICADO]
  - El script pone el esquema, activa el modo STRICT y crea las políticas (`infra/cloudformation/scripts/70-avp-apply-schema-policies.ps1:113-207`).
  - Los tres micros usan `IsAuthorizedWithToken`, que **requiere identity source**, así que las llamadas fallarían. [INFERIDO: requisito documentado de AVP]
- **IAM:** los roles de identity, account y ledger tienen `verifiedpermissions:IsAuthorizedWithToken` con `Resource "*"` (`infra/cloudformation/50-compute-ecs/template.yml:201-274`). [VERIFICADO]

### 5.2 Uso en código

| Servicio | ¿Llama a AVP? | Acción / tipo enviados | ¿Coincide con el esquema? | Decisión aplicada |
|---|---|---|---|---|
| identity | Código real (`IDN/infra/security/avp/AvpAuthorizer.java:39-68`), pero **ninguna ruta en AUTHZ** | `ImaginaryBank::Action::identity.*`, `Registration`, `Session`, `System` | **No** | — (nunca se ejecuta) [VERIFICADO] |
| account | Sí, en todas las rutas AUTHZ (`ACC/infra/security/avp/AvpAuthorizer.java:39-67`) | `ImaginaryBank::Action::accounts:*`. El principal no se envía (línea comentada `:47`). | **Sí** | **DENY no aplicado**: solo se registra en log y la petición continúa (`ACC/infra/security/grpc/AuthzServerInterceptor.java:269-284`). La auditoría registra `ALLOW` igualmente (`:280-282`). [VERIFICADO] |
| ledger | Sí, en todas las rutas AUTHZ (`LED/infra/security/avp/client.go:55-77`) | `ActionType "Action"` **sin namespace**, IDs `ledger.payments.post` y otros. Solo identificador de recurso, sin `Entities`. | **No** | DENY aplicado (`PermissionDenied`, `LED/infra/security/grpc/authz_interceptor.go:198-203`). Con el esquema actual siempre sería DENY. [VERIFICADO / INFERIDO] |
| BFF | No | — | — | — [NO ENCONTRADO] |

Contexto que se envía a AVP:

| Servicio | Contexto | Evidencia |
|---|---|---|
| account | `customer_id`, `channel`, `ip_hash`, `user_agent_hash`, `idempotency_key`, `mfa_verified` | `ACC/infra/security/grpc/AuthzServerInterceptor.java:211-221` |
| ledger | `correlation_id`, `route_template`, `ip_hash`, `ua_hash`, `customer_id`, `idempotency_key` | `LED/infra/security/grpc/authz_interceptor.go:167-181` |

**Ninguno envía** importe, umbral, `auth_time` ni estado de cuenta. En account, el estado de cuenta viaja como atributo del recurso, pero ninguna política lo usa. [VERIFICADO]

### 5.3 Comportamiento si falla el componente de autorización

| Servicio | AVP con error o timeout | DENY explícito | JWKS caído | Evidencia |
|---|---|---|---|---|
| identity | Fail-closed (`PERMISSION_DENIED avp_error`), aunque nunca se ejecuta | Fail-closed | `UNAUTHENTICATED`; en el arranque, el servicio no arranca | `IDN/infra/security/grpc/AuthzServerInterceptor.java:184-216` [VERIFICADO] |
| account | **Fail-closed** (`PERMISSION_DENIED avp_error`) | **Fail-open** (se permite la operación) | `UNAUTHENTICATED` | `ACC/infra/security/grpc/AuthzServerInterceptor.java:250-284` [VERIFICADO] |
| ledger | **Fail-closed** (`codes.Internal`; mensaje oculto fuera de dev) | Fail-closed | `UNAUTHENTICATED`; los kid en caché sirven hasta 10 min | `LED/infra/security/grpc/authz_interceptor.go:189-203`, `LED/infra/security/jwt/validator.go:166-214` [VERIFICADO] |
| BFF | Convierte cualquiera de los anteriores en **502** | 502 | — | `BFF/api/rest/v1/web/feature/accounts/strict.go:107-113` [VERIFICADO] |

No hay timeout explícito en los clientes AVP; se usan los valores por defecto del SDK (`ACC/infra/config/AwsClientsConfig.java:22-28`). No hay caché de decisiones. [VERIFICADO / NO ENCONTRADO]

## 6. Roles y permisos existentes

- `cognito:groups` se lee como roles en los tres micros y en el BFF, pero **ninguna decisión los usa**. Tampoco hay grupos en Cognito ni políticas que usen `User.roles`. [VERIFICADO / NO ENCONTRADO]
- La web no tiene guards por rol (`WEB/app.routes.ts:5-24`). [VERIFICADO]
- El contrato OpenAPI declara un tag `Admin` sin operaciones. La web conserva `admin.service.ts` (`/api/v1/admin/sandbox/topups`), que el BFF no implementa. [VERIFICADO]
- Estado del usuario:

  | Componente | Comportamiento | Evidencia |
  |---|---|---|
  | BFF | Si `user_status` no es `ACTIVE` devuelve 403, pero **si llega vacío deja pasar** | `BFF/api/rest/middleware/auth_session.go:90-93` |
  | identity | Reactiva `LOCKED`/`DISABLED` en el login | `IDN/application/service/OidcAuthService.java:151` |
  | account | `customer.status=suspended` no tiene efecto | — |

  [VERIFICADO]
