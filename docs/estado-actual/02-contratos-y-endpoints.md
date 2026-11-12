# 02 · Contratos y endpoints

Convenciones y alias: ver [00-INDICE](00-INDICE.md). `OAS` = `bank-bff-openapi/openapi/bff.openapi.yaml`.

## 1. Fuentes de contrato

| Contrato | Versión | Consumidores | Notas |
|---|---|---|---|
| `bank-bff-openapi` (OpenAPI 3.0.3) | tag v0.0.20 (HEAD bf04392). `info.version` vale 1.0.0 (`OAS:4`). | BFF fija 0.0.20 (`bff/package.json:5`, oapi-codegen). Web usa `^0.0.20` (`imaginarybank-web/package.json:33`, openapi-generator 7.18.0). | Solo hay dos securitySchemes de tipo `apiKey`: la cookie `session_id` y la cabecera `X-CSRF-Token` (`OAS:29-39`). **No hay `security` global.** El BFF usa la cookie `bff_sid` / `__Host-bff_sid` (`BFF/security/cookies.go:27-30`). [VERIFICADO] |
| `bank-contracts` (protobuf, buf v2) | tags v0.1.0, v0.2.0 y v0.3.0 (HEAD aa4ea3e) | account e identity usan Maven 0.3.0 (`micro/*/pom.xml`). ledger usa Go v0.3.0 (`micro/ledger/go.mod`). **El BFF no lo usa**: genera desde `buf.build/imaginarybank/bank:v0.0.19` (`bff/contracts/proto.ref.txt:1`). | No hay opciones de seguridad en los proto; solo comentarios del tipo "INTERNAL… mTLS/API key" (`bank-contracts/proto/bank/accounts/v1/accounts.proto:44-47`). Al código generado del BFF le faltan `OpenAccountWithOpeningBonus`, `ResolvePrincipal` y `CreditAccountSystem`. [VERIFICADO] |

## 2. Endpoints REST del BFF (contrato frente a implementación)

Las 17 operaciones del contrato coinciden con las 17 rutas registradas (`BFF/api/rest/gen/openapi/openapi.gen.go:1613-1662`; el strict server obliga a implementarlas todas). [VERIFICADO]

Leyenda de la columna "Token":

| Valor | Significado |
|---|---|
| **C** | Cookie de sesión. `AuthSession` llama a `Identity.GetSessionInfo` y obtiene el access token. |
| **C+X** | Cookie de sesión más CSRF. |
| **—** | Pública. |

El navegador nunca maneja JWT.

| Método | Ruta | Línea OAS | RPC downstream | Consumidor | Identificadores (origen) | Token | Comprobación de autorización (evidencia) |
|---|---|---|---|---|---|---|---|
| GET | `/api/v1/auth/oidc/start` | 602 | identity `StartOidcLogin` | Web (`WEB/core/auth/auth.service.ts:22-26`) | `redirect` (query) | — | Solo la allowlist de redirección (`BFF/security/redirect_policy.go:41-76`). [VERIFICADO] |
| GET | `/api/v1/auth/oidc/callback` | 650 | identity `CompleteOidcLogin` | Cognito (redirect) | `code`, `state` (query) | — | Identity valida state, PKCE y nonce. El BFF no liga el `state` al navegador, lo que permite un posible login CSRF (`BFF/api/rest/server/auth_strict.go:15-102`). [VERIFICADO / INFERIDO] |
| POST | `/api/v1/session/logout` | 710 | identity `LogoutSession` | Web | sid (cookie) | C+X | Solo sesión. [VERIFICADO] |
| GET | `/api/v1/session/csrf` | 755 | — | Web | — | C | Solo sesión. [VERIFICADO] |
| POST | `/api/v1/session/refresh` | 798 | identity `RefreshSession` | Ninguno en la UI | sid | C+X | Solo sesión. [VERIFICADO] |
| GET | `/api/v1/session` | 846 | — (datos del contexto) | Web (guards) | sesión | C | Exige `sub` (`BFF/api/rest/v1/web/feature/session/strict.go:15-21`). [VERIFICADO] |
| GET | `/api/v1/accounts` | 893 | account `ListAccounts` | Web `home` | `customer_id` **de la sesión** | C | Sin IDOR posible: el cliente sale de la sesión (`BFF/.../feature/accounts/strict.go:18-25`). Account además lo fuerza al principal (`ACC/infra/in/grpc/validation/AccountsGrpcValidators.java:18-30`). [VERIFICADO] |
| PATCH | `/api/v1/accounts/{id}/limits` | 950 | account `PatchAccountLimits` | Ninguno en la UI | `accountId` (path, string) | C+X | **Ninguna en el BFF.** En account, AVP evalúa pero el DENY no se aplica (`ACC/infra/security/grpc/AuthzServerInterceptor.java:269-284`). El contrato dice "ownership por policy/AVP" (`OAS:960`). [VERIFICADO] |
| GET | `/api/v1/accounts/lookup` | 1028 | account `GetAccountByNumber` | Ninguno en la UI | `account_number` (query) | C | **Ninguna.** Devuelve `account_id` y nombre del titular de cualquier cuenta (`BFF/.../accounts/strict.go:131-185`). En account la ruta no está registrada, lo que provoca un NPE y un fallo (`ACC/infra/security/grpc/AuthzServerInterceptor.java:89-91`). [VERIFICADO / INFERIDO] |
| GET | `/api/v1/accounts/{id}/statement` | 1100 | ledger `ListAccountStatement` | Web `home` | `accountId` (path, uuid) | C | **Ninguna en el BFF** (`BFF/.../accounts/strict.go:191-317`). Ledger no resuelve el propietario (`LED/infra/security/authz/resource_templates.go:36-41`). [VERIFICADO] |
| POST | `/api/v1/onboarding/intents` | 1184 | identity `StartRegistration` | Web `register` | — (body con PII) | — | Pública, sin CSRF ni captcha. [VERIFICADO] |
| POST | `/api/v1/onboarding/kyc/confirm` | 1238 | identity `ConfirmRegistrationKyc` | Web `register` | `registration_id` (body) | — | Basta con poseer el `registration_id`, sin comprobar el estado previo (`IDN/application/service/OnboardingService.java:81-102`). [VERIFICADO] |
| POST | `/api/v1/onboarding/activate` | 1292 | identity `ActivateRegistration` | Web `register` | `registration_id` (body) | — en el OAS. Identity exige JWT, pero el BFF no tiene sesión en esta ruta pública. | **Incoherencia:** identity es `AUTHN_ONLY` y comprueba que el registro sea del usuario (`IDN/application/service/OnboardingService.java:233-259`). El BFF no ejecuta `AuthSession` en rutas públicas, así que no hay bearer y la llamada probablemente falla con UNAUTHENTICATED (`BFF/client/grpc/outgoing_metadata.go:59-61`). Además provoca un panic si falta `accepted_terms` (`BFF/client/grpc/identity.go:241`). [VERIFICADO / INFERIDO] |
| POST | `/api/v1/beneficiaries` | 1354 | account `ListAccounts` (lectura) | Ninguno en la UI | `destination_account` (body) | C+X | Comprueba que el destino sea **propio**. Es un mock que no persiste nada (`BFF/.../feature/payments/strict.go:43-58`). [VERIFICADO] |
| POST | `/api/v1/payments` | 1414 | account `InternalAccountsService/ValidateAccountsAndLimits` y luego ledger `PostPayment` | Web `home` | `source_account`, `destination_account`, `amount`, `currency` (body); `Idempotency-Key` (cabecera) | C+X | **No comprueba que el origen pertenezca al cliente** (`BFF/.../payments/strict.go:67-154`). `initiated_by` sale de la sesión. **El BFF invoca una RPC interna.** [VERIFICADO] |
| GET | `/api/v1/payments/{id}` | 1490 | ledger `GetPayment` | Ninguno en la UI | `paymentId` (path) | C | Solo exige `customer_id` no vacío, sin comprobar el propietario (`BFF/.../payments/strict.go:156-229`). Ledger tampoco lo comprueba (bug de tipos en `LED/domain/model/payment.go:31` y `LED/infra/security/authz/resource_resolver_reflect.go:47`). [VERIFICADO] |
| PATCH | `/api/v1/profile` | 1542 | account `PatchCustomer` | Ninguno en la UI | `customer_id` de la sesión | C+X | El cliente sale de la sesión, sin IDOR desde el BFF (`BFF/.../feature/profile/strict.go:19,38-39`). [VERIFICADO] |

**Comportamiento de error transversal.** Todos los handlers convierten cualquier error gRPC (`PermissionDenied`, `Unauthenticated`, `NotFound`…) en **502 `UPSTREAM_ERROR`** (por ejemplo `BFF/.../accounts/strict.go:107-113` y `BFF/.../payments/strict.go:107-142`). `httperr.MapGrpc` existe pero no se usa (`BFF/api/rest/httperr/httperr.go:58-80`). Por tanto **un DENY de autorización no se distingue de una caída del sistema**. [VERIFICADO]

**Diferencias entre contrato e implementación (REST)** [VERIFICADO]:
- Nombre de la cookie: `session_id` en el OAS frente a `bff_sid` en el código.
- El contrato no define 428, step-up ni campo de motivo de denegación. `ErrorResponse` solo tiene `{code, message, details}` (`OAS:80-93`).
- El contrato solo declara 404 en `lookup`, mientras que la implementación nunca devuelve 403 ni 404 de negocio.
- `Account.status` es un string sin enum.
- `CreatePaymentRequest.amount` es un string sin `pattern` (`OAS:484-496`).
- La web conserva servicios generados que ya no están en el contrato: `admin.service.ts` y `webauthn.service.ts`.

## 3. Endpoints gRPC (bank-contracts frente a implementación)

Modos de autorización por servicio:

| Servicio | Modos | Evidencia |
|---|---|---|
| account | `PUBLIC` / `AUTHZ`; rutas no registradas provocan un NPE | `ACC/infra/security/authz/RouteAuthzRegistry.java:17-50` |
| identity | `PUBLIC` / `AUTHN_ONLY`; ninguna ruta en `AUTHZ` | `IDN/infra/security/authz/RouteAuthzRegistry.java:16-40` |
| ledger | `AUTHZ` / `SERVICE_ONLY`; rutas no registradas se deniegan | `LED/infra/security/authz/route_registry.go:17-43` |

"Exp." indica la exposición: **PÚB** = la invoca el BFF; **INT** = se invoca entre servicios.

### 3.1 identity (`bank-contracts/proto/bank/identity/v1/identity.proto`)

| RPC (línea proto) | Exp. | Consumidor | Identificadores | Token | Autorización real |
|---|---|---|---|---|---|
| `StartRegistration` (:18) | PÚB | BFF | — | No (PUBLIC) | Ninguna. [VERIFICADO] |
| `ConfirmRegistrationKyc` (:21) | PÚB | BFF | `registration_id` | No | Ninguna. La clave S3 se limita a `staging/<regId>/` (`IDN/infra/out/storage/S3PresignedKycStorage.java:148-161`). [VERIFICADO] |
| `ActivateRegistration` (:23) | PÚB | BFF | `registration_id` | Sí (AUTHN_ONLY) | Comprueba en el servicio que el `sub` corresponda a `reg.identityId` (`IDN/application/service/OnboardingService.java:233-259`). Ese vínculo se crea por coincidencia de email sin `email_verified` (`IDN/application/service/OidcAuthService.java:222-240`). [VERIFICADO] |
| `StartOidcLogin` (:28) | PÚB | BFF | — | No | — |
| `CompleteOidcLogin` (:31) | PÚB | BFF | `code`, `state` | No | State de un solo uso, PKCE y nonce (`IDN/application/service/OidcAuthService.java:110-201`). [VERIFICADO] |
| `RefreshSession` (:34) | PÚB | BFF | `session_id` | No (el comentario dice AUTHN_ONLY) | **Ninguna.** El `session_id` funciona como credencial al portador y la respuesta incluye un access token nuevo (`IDN/infra/in/grpc/OidcAuthGrpcService.java:172-185`). [VERIFICADO] |
| `LogoutSession` (:35) | PÚB | BFF | `session_id` | No | Ninguna. [VERIFICADO] |
| `GetSessionInfo` (:36) | PÚB | BFF | `session_id` | No | **Ninguna. Devuelve el access token de Cognito descifrado** (`IDN/application/service/SessionService.java:73-89`). [VERIFICADO] |
| `ResolvePrincipal` (:41) | INT | account | — (los datos salen del token) | Sí (AUTHN_ONLY) | Acepta cualquier access token válido del pool. Devuelve `ACTIVE` aunque la identidad no exista (`IDN/infra/in/grpc/PrincipalGrpcService.java:45-57`). [VERIFICADO] |

### 3.2 account (`bank-contracts/proto/bank/accounts/v1/accounts.proto`)

| RPC (línea proto) | Exp. | Consumidor | Identificadores | Token / modo | Autorización real |
|---|---|---|---|---|---|
| `CreateCustomer` (:22) | PÚB+INT | identity (el cliente del BFF existe pero no se usa) | — | **PUBLIC** | Ninguna. [VERIFICADO] |
| `PatchCustomer` (:25) | PÚB | BFF `/profile` | `id` (customer) | AUTHZ `customers:patch` | **No hay comprobación en el servicio.** No existe política Cedar y el DENY no se aplica, así que cualquier token puede cambiar `risk_segment` o `customer_status` de cualquier cliente (`ACC/application/service/CustomerService.java:50-73`). [VERIFICADO] |
| `ListAccounts` (:30) | PÚB | BFF | `customer_id` | AUTHZ `accounts:read` | **Se fuerza el cliente del principal** (`ACC/infra/in/grpc/validation/AccountsGrpcValidators.java:18-30`). [VERIFICADO] |
| `CreateAccount` (:33) | PÚB | ninguno (sin ruta en el BFF) | `customer_id` | **PUBLIC** | Ninguna. Devuelve el `account_id` y número de cuenta de un cliente ajeno si ya existe (`ACC/application/service/AccountService.java:70-78`). [VERIFICADO] |
| `GetAccountBalances` (:36) | PÚB | ninguno (el saldo llega dentro de `ListAccounts`) | `id` (account) | AUTHZ `accounts:balances_read` | AVP recibe `owner_customer_id`, pero **el DENY no se aplica** y el servicio no compara (`ACC/application/service/AccountService.java:159-164`). [VERIFICADO] |
| `PatchAccountLimits` (:39) | PÚB | BFF | `id` | AUTHZ `accounts:limits_patch` | Igual que la anterior. Además falla siempre por la idempotency-key (interceptor no registrado, `ACC/infra/in/grpc/AccountsGrpcService.java:163`). [VERIFICADO / INFERIDO] |
| `GetAccountByNumber` (:41) | PÚB | BFF lookup | `account_number` | **No registrada → NPE** | Fallo por accidente. [VERIFICADO] |
| `OpenAccountWithOpeningBonus` (:47) | INT | identity | `customer_id` | **PUBLIC** | Ninguna. Desencadena el abono de 50 USD en ledger con token de servicio. [VERIFICADO] |
| `ValidateAccountsAndLimits` (:49) | INT | ledger **y BFF** | `source_account_id`, `destination_account_id` | AUTHZ `accounts:validate` (token de usuario) | La plantilla llama a `getId()`, que no existe en este mensaje, así que **siempre deniega con `resource_resolve_error`** fuera del perfil `local` (`ACC/infra/security/authz/AccountResourceTemplates.java:65-67`). No compara origen con principal (`ACC/application/service/AccountService.java:177-234`). [INFERIDO / VERIFICADO] |
| `ReserveHold` (:52) | INT | ledger | `id`, `hold_id` | AUTHZ `accounts:hold_reserve` (token de usuario) | DENY no aplicado. Comprueba que la cuenta esté `active`, pero no el titular (`ACC/application/service/AccountService.java:237-250`). [VERIFICADO] |
| `ReleaseHold` (:55) | INT | ledger | `id`, `hold_id` | AUTHZ `accounts:hold_release` | DENY no aplicado. No comprueba estado ni `hold_id` (`ACC/application/service/AccountService.java:252-262`). [VERIFICADO] |
| `BatchGetAccountSummaries` (:57) | INT | ledger (sin metadata) | `account_ids[]` | **No registrada → NPE** | Fallo; el extracto se degrada sin aviso (`LED/application/service/ledger_service.go:491-495`). [VERIFICADO] |

### 3.3 ledger (`bank-contracts/proto/bank/ledgerpayments/v1/ledgerpayments.proto`)

| RPC (línea proto) | Exp. | Consumidor | Identificadores | Token / modo | Autorización real |
|---|---|---|---|---|---|
| `PostPayment` (:10) | PÚB | BFF | `source_account_id`, `destination_account_id`, `idempotency_key` | AUTHZ `ledger.payments.post` (token de usuario) | Recurso AVP `Payment/"new"`, **sin propietario, cuenta ni importe** (`LED/infra/security/authz/resource_templates.go:27-28`). La acción no existe en el esquema, así que AVP deniega. [VERIFICADO / INFERIDO] |
| `GetPayment` (:11) | PÚB | BFF | `payment_id` | AUTHZ `ledger.payments.get` | El propietario nunca se resuelve (bug `*uuid.UUID`) y no se filtra (`LED/application/service/payment_service.go:260-289`). [VERIFICADO] |
| `CreditAccountSystem` (:16) | INT | account (bono) | `account_id` | SERVICE_ONLY | `client_id` en lista blanca, `scope` y `token_use=access` (`LED/infra/security/jwt/validator.go:339-385`). **No pasa por AVP.** `reason` debe ser `registration_bonus`. [VERIFICADO] |
| `CreditAccount` (:17) | INT (según comentarios) | nadie lo invoca: los clientes de identity y del BFF existen pero no se usan | `account_id`, `customer_id` | **AUTHZ con token de usuario** | **Crea dinero** desde `GL_SYSTEM_FUND` sin límite de motivo ni importe (`LED/application/service/ledger_service.go:31-178`). Solo lo frenaría una política AVP inexistente. [VERIFICADO] |
| `ListAccountJournalEntries` (:18) | PÚB | ninguno vía REST | `account_id` | AUTHZ | Sin propietario. [VERIFICADO] |
| `ListAccountStatement` (:20) | PÚB | BFF statement | `account_id` | AUTHZ `ledger.statement.list` | Sin propietario y sin entidad de recurso enviada a AVP (`LED/infra/security/authz/authz_interceptor.go:167-181`). [VERIFICADO] |

### 3.4 ops (`bank-contracts/proto/bank/ops/v1/ops.proto`)

`AuditService.GetAuditEventsByTrace` (:23) y `NotificationsService.GetNotificationPrefs` (:29) no tienen ni servidor ni cliente. [NO ENCONTRADO] El `go_package` apunta a `github.com/imaginarybank/banking-contracts` (`ops.proto:11`), que no es el módulo correcto.

## 4. Diferencias entre contrato e implementación (gRPC) [VERIFICADO]

- **Estado de cuenta.** El proto usa vocabularios distintos: `active|blocked|closed` (`accounts.proto:298`) y `active|frozen` (`:333`). La base de datos solo admite `active|frozen` (`micro/account/src/main/resources/db/migration/V1__init.sql:51`).
- **Respuestas de holds.** `ReserveHoldResponse.hold_id/status` y `ReleaseHoldResponse.status` nunca se rellenan (`ACC/infra/in/grpc/InternalAccountsGrpcService.java:96-112`).
- **`account_number` en `ListAccounts`.** `AccountView.account_number` no se rellena en esa respuesta (`ACC/infra/in/grpc/AccountsGrpcService.java:74-86`).
- **Estados de registro.** `RegistrationState` no incluye `KYC_CONFIRMED` ni `ACTIVATING`, que se devuelven como `UNSPECIFIED` (`IDN/infra/in/grpc/OnboardingGrpcService.java:94-111`).
- **`ResolvePrincipal`.** El campo `purpose` se ignora.
- **`GetSessionInfo`.** El usuario se devuelve vacío y sin roles.
- **Pasos del pago.** `PaymentStep.step` usa `VALIDATE|RESERVE…` en el proto (`ledgerpayments.proto:42`), pero el código usa `reserve_hold` y `post_ledger`.
- **Importes y errores.** Los importes son siempre `string` decimal, sin tipo Money; la precisión (20,6) solo se indica en un comentario (`ledgerpayments.proto:28`). No se usa `google.rpc.Status`.
- **`ValidateAccountsAndLimits`.** Devuelve `ok` y `reason` dentro de la respuesta.
- **Identidad del actor.** Llega en campos que controla el llamante (`initiated_by`, `x_subject`). Ledger da prioridad al `sub` del JWT (`LED/application/service/payment_service.go:33-37`).
