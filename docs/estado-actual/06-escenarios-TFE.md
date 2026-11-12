# 06 · Escenarios del TFE: comportamiento actual según el código

Convenciones y alias: ver [00-INDICE](00-INDICE.md).

> Análisis **estático**: no se ha ejecutado ni atacado nada.

**Supuestos comunes.** Si cambian, el resultado puede cambiar.

| # | Supuesto |
|---|---|
| S1 | Account y ledger se ejecutan con un perfil distinto de `local`. Es el caso por defecto en account y el único posible en ledger. |
| S2 | Identity se ejecuta con su perfil por defecto, `local`, que significa **sin autenticación ni autorización** (`micro/identity/src/main/resources/application.yml:5`). |
| S3 | El policy store de AVP contiene **solo** el esquema y las políticas del repositorio y tiene un identity source configurado. Si no lo tiene, todas las llamadas `IsAuthorizedWithToken` lanzan una excepción y account y ledger deniegan con `avp_error`. No se ha podido verificar qué hay desplegado. |
| S4 | Los access tokens de Cognito no llevan `amr` ni `custom:customer_id`. [INFERIDO] |

Los `.env` no están en el repositorio, así que S1–S3 no se pueden confirmar.

**Leyenda de resultados**

| Etiqueta | Significado |
|---|---|
| **Correcto** | El resultado coincide con el esperado y por la razón adecuada. |
| **Correcto por accidente** | Coincide, pero por un bug o por falta de política, no por un control de diseño. |
| **Incorrecto** | No coincide. |
| **Indeterminado** | Depende de configuración no verificable. |

---

### E1. El cliente consulta su cuenta → esperado: PERMITIR

- **Ruta:** `GET /api/v1/accounts` → account `ListAccounts`.
- **Hoy:**
  - El BFF toma el `customer_id` de la sesión (`BFF/api/rest/v1/web/feature/accounts/strict.go:18-25`).
  - Account valida el JWT, resuelve el cliente con `ResolvePrincipal` y lo **fuerza** al del principal (`ACC/infra/in/grpc/validation/AccountsGrpcValidators.java:18-30`).
  - AVP evalúa `01_accounts_read_own` con `context.customer_id == owner`, lo que da ALLOW.
- **Resultado:** **PERMITIR**. [INFERIDO; **indeterminado** si el store no tiene identity source, en cuyo caso se deniega con `avp_error` y el cliente ve un 502]
- **Falta:**
  - Que la política compare `principal`, y no un valor del contexto.
  - Configurar el identity source.
  - Que el BFF devuelva el código HTTP correcto.

### E2. Consultar una cuenta ajena cambiando el ID → esperado: DENEGAR

- **Vía BFF:**
  - `/accounts` no acepta IDs, así que no es manipulable.
  - `PATCH /accounts/{id}/limits` con un id ajeno: account recibe el DENY de AVP pero **no lo aplica** (`ACC/infra/security/grpc/AuthzServerInterceptor.java:269-284`). La operación acaba fallando de todos modos por la idempotency-key (`ACC/infra/in/grpc/AccountsGrpcService.java:163`), lo que es correcto solo por accidente.
  - `GET /accounts/lookup` devuelve el `account_id` y el nombre del titular de cualquier cuenta, aunque en account esa ruta no registrada provoca un NPE (`ACC/infra/security/grpc/AuthzServerInterceptor.java:89-91`).
- **Vía directa a account** (`GetAccountBalances`, puerto 19092 publicado en el host; `docker-compose.yml:142`): con un token válido y el `id` de otra cuenta, AVP devuelve DENY. El interceptor solo escribe `log.warn AUTHZ_DENY` y continúa, y el servicio devuelve los saldos (`ACC/application/service/AccountService.java:159-164`).
- **Resultado:** **INCORRECTO: se permite.** [VERIFICADO en código; el efecto en ejecución es INFERIDO]
- **Falta:**
  - Aplicar el DENY: cerrar la llamada con `PERMISSION_DENIED`.
  - Comprobación defensiva de titularidad en el servicio (`enforceSelfOnlyCustomer` existe pero no se invoca: `ACC/infra/in/grpc/validation/AccountsGrpcValidators.java:79-88`).
  - Exponer en el BFF un endpoint de saldo por cuenta para poder probar el escenario de punta a punta.

### E3. Consultar sus movimientos → esperado: PERMITIR

- **Ruta:** `GET /accounts/{id}/statement` → ledger `ListAccountStatement`.
- **Hoy:** ledger pide a AVP `Action::"ledger.statement.list"` sobre `Account/{id}`. Ni ese tipo de acción ni ese ID existen en el esquema y no hay ninguna política para ellos (`LED/infra/security/avp/client.go:49`, `LED/infra/security/authz/route_registry.go:41-43`). AVP deniega, ledger devuelve `PermissionDenied` y el BFF lo convierte en 502.
- **Resultado:** **INCORRECTO: se deniega** (falso negativo). [INFERIDO por S3]
- **Falta:**
  - Alinear las acciones de ledger con el esquema: usar el namespace `ImaginaryBank::Action` y añadir una acción `statement:read`.
  - Resolver el propietario de la cuenta. Ledger no lo conoce, así que hay que preguntarlo a account o enviar la entidad.
  - Una política de titularidad.

### E4. Movimientos de una cuenta ajena → esperado: DENEGAR

- **Hoy:** se deniega por el mismo motivo que E3. No hay ninguna comprobación de titularidad: el BFF no la hace (`BFF/api/rest/v1/web/feature/accounts/strict.go:191-317`) y ledger no envía el propietario (`LED/infra/security/authz/resource_templates.go:36-41`). Si se añadiera una política permisiva para la acción, **se podrían leer movimientos ajenos**, porque la consulta filtra solo por `counterparty_ref` (`LED/infra/out/persistence/gorm/adapter/journal_repo.go:84-104`).
- **Resultado:** **Correcto por accidente.**
- **Falta:** lo mismo que en E3, más un caso de prueba negativo.

### E5. Transferencia desde cuenta propia a otro cliente, bajo umbral y con autenticación reciente → esperado: PERMITIR

- **Hoy:**
  - La UI solo ofrece transferencias entre cuentas propias (`WEB/features/home/home.html:256-269`), pero la API acepta cualquier destino.
  - El BFF llama a `ValidateAccountsAndLimits`. En account, la plantilla `ACCOUNT_BY_ID` llama a `getId()`, que no existe en ese mensaje, así que da `resource_resolve_error` y `PERMISSION_DENIED` (`ACC/infra/security/authz/AccountResourceTemplates.java:65-67`; `bank-contracts/proto/bank/accounts/v1/accounts.proto:235-240`). El cliente recibe un 502.
  - Aunque ese paso pasara, ledger enviaría `Action::"ledger.payments.post"`, que tampoco tiene política, y AVP denegaría.
  - Además, ni el umbral ni la "autenticación reciente" existen.
- **Resultado:** **INCORRECTO: se deniega** (falso negativo; el flujo de pago está roto). [INFERIDO]
- **Falta:**
  - Corregir la plantilla de `ValidateAccountsAndLimits` para que use `source_account_id`.
  - Alinear las acciones de ledger con el esquema.
  - Enviar el importe, `auth_time` o `amr` y el estado a AVP.
  - Escribir la política de umbral.
  - Hacer que identity exponga `auth_time` o `amr` al BFF.

### E6. Transferencia desde una cuenta ajena → esperado: DENEGAR

- **Hoy:**
  - El BFF no comprueba la titularidad de `source_account` (`BFF/api/rest/v1/web/feature/payments/strict.go:100-135`).
  - Account no compara el origen con el principal (`ACC/application/service/AccountService.java:177-234`) y no aplica el DENY.
  - Ledger usa `Payment/"new"` sin propietario (`LED/infra/security/authz/resource_templates.go:27-28`).
  - Hoy se deniega solo por los mismos bugs que en E5. **Si se arreglan esos bugs sin añadir controles, se permitiría.**
  - La política `02_payments_create_mfa.cedar` sí prevé `source_account_owner_customer_id`, pero nadie envía ese atributo.
- **Resultado:** **Correcto por accidente** (latente: incorrecto).
- **Falta:**
  - Que ledger o account resuelvan `source_account_owner_customer_id` y lo envíen como atributo de la entidad `Payment`.
  - Aplicar el DENY en account.
  - Una comprobación defensiva en el BFF.

### E7. Transferencia por encima del umbral sin autenticación reciente ni MFA → esperado: DENEGAR CON MOTIVO

- **Hoy:**
  - No hay umbral en el código, el esquema ni las políticas [NO ENCONTRADO].
  - `02_payments_create_mfa.cedar` no evalúa `mfa_verified`.
  - `amr` no llega a AVP en ledger, que solo lo usa para auditoría (`LED/infra/security/jwt/validator.go:143-150`).
  - Nadie lee `auth_time`.
  - Cognito no tiene step-up por transacción.
  - El BFF convierte toda denegación en 502 sin motivo, y el contrato no tiene códigos 428 ni `STEP_UP_REQUIRED`.
- **Resultado:** **Correcto por accidente** (se deniega por los bugs de E5) y **sin motivo**. Si se corrigen los bugs, **se permitiría**.
- **Falta (núcleo del TFE):**
  - Un atributo de umbral en una entidad o en el contexto.
  - Contexto con `amount`, `auth_time`/`auth_age_seconds` y `mfa`.
  - Una política `forbid` con `@id`/`@reason`.
  - Un endpoint de step-up (re-login con `max_age` o `prompt=login`).
  - Mapear el deny a un 401/403 con código.

### E8. Transferencia desde una cuenta bloqueada o inactiva → esperado: DENEGAR CON MOTIVO

- **Hoy:**
  - El estado se modela como `frozen` (`micro/account/src/main/resources/db/migration/V1__init.sql:51`).
  - `ValidateAccountsAndLimits` devuelve `ok=false` con el motivo "source not active" (`ACC/application/service/AccountService.java:196-197`), y el BFF devolvería **400 con `reason`** (`BFF/api/rest/v1/web/feature/payments/strict.go:114-124`). Pero hoy la llamada no llega al servicio por el bug de la plantilla de E5.
  - En ledger, el mismo `ok=false` se convierte en `Internal` sin motivo (`LED/application/service/payment_service.go:65-81`).
  - No hay RPC para bloquear cuentas: solo se puede hacer por SQL.
  - Cedar recibe `status` como atributo del recurso (`ACC/infra/security/authz/AccountResourceTemplates.java:76`), pero ninguna política lo usa.
- **Resultado:** **Correcto por accidente.** El motivo trazable existiría solo en la vía BFF → account, si se corrige la plantilla.
- **Falta:**
  - Una política `forbid` sobre `resource.status != "active"`, extendida también a `customer.status`.
  - Propagar el motivo hasta el cliente.
  - Una forma de bloquear cuentas: RPC de administración o script de prueba.

### E9. Sin token, o con token manipulado o caducado → esperado: DENEGAR

- **Vía BFF:** sin cookie, o con una cookie inválida, da 401 (`BFF/api/rest/middleware/auth_session.go:198-207`). El navegador no maneja JWT. Esto es **correcto**.
- **Vía directa a los micros:**

  | Servicio / rutas | Comportamiento | Evidencia |
  |---|---|---|
  | account, rutas AUTHZ | `UNAUTHENTICATED` si falta el token o no es válido (firma, `iss`, `exp`, `token_use`, `client_id`) | `ACC/infra/security/grpc/AuthzServerInterceptor.java:101-115` |
  | ledger | Valida firma y `exp`; `iss` y `aud` solo si están configurados | `LED/infra/security/jwt/validator.go:107-121` |
  | account, rutas **PUBLIC** (`CreateCustomer`, `CreateAccount`, `OpenAccountWithOpeningBonus`) | **Aceptan cualquier petición** | `ACC/infra/security/authz/RouteAuthzRegistry.java:28-34,48-49` |
  | identity, perfil `local` | Acepta todo | — |
  | identity, `GetSessionInfo` / `RefreshSession` | No usan token; basta con el `session_id` | — |

- **Resultado:** **Correcto en el borde; parcial en los micros.** [VERIFICADO]
- **Falta:**
  - Eliminar las rutas PUBLIC internas.
  - Perfil seguro por defecto en identity.
  - Hacer obligatorios `iss`/`aud` (o `client_id`) en ledger.
  - mTLS o red privada estricta.

### E10. El cliente invoca una operación interna no expuesta → esperado: DENEGAR

| Operación | Resultado actual | Evidencia |
|---|---|---|
| ledger `CreditAccountSystem` con token de usuario | **Deniega** (falla la lista blanca de `client_id` y el `scope`) → correcto | `LED/infra/security/jwt/validator.go:353-382` |
| ledger `CreditAccount` (crea dinero) con token de usuario | Ruta AUTHZ que admite usuarios. Se deniega solo porque no hay política → **correcto por accidente** | `LED/infra/security/authz/route_registry.go:28-30`, `LED/application/service/ledger_service.go:31-178` |
| account `ReserveHold` / `ReleaseHold` con token de usuario sobre cualquier cuenta | No hay política, pero **el DENY no se aplica** → **INCORRECTO: se permite** (bloquear o liberar fondos) | `ACC/infra/security/grpc/AuthzServerInterceptor.java:269-284`, `ACC/application/service/AccountService.java:237-262` |
| account `OpenAccountWithOpeningBonus` sin token | **PUBLIC** → **INCORRECTO: se permite** | `ACC/infra/security/authz/RouteAuthzRegistry.java:48-49` |
| account `ValidateAccountsAndLimits` | **El propio BFF la invoca** desde el borde | `BFF/api/rest/v1/web/feature/payments/strict.go:101-106` |

- **Resultado:** **Mixto**: ledger deniega y account permite.
- **Falta:**
  - Separar las rutas internas: modo `SERVICE_ONLY` en account, aceptando solo tokens M2M con scope.
  - Que el BFF no llame a RPC internas.
  - Exigir identidad de servicio con mTLS o client_credentials y scope por acción.

### E11. Un servicio interno llama a ledger para algo ajeno a su función → esperado: DENEGAR

- **Hoy:**
  - Los tokens M2M (sin `customer_id`) en rutas AUTHZ de ledger se deniegan por `customer_link_missing` (`LED/infra/security/grpc/authz_interceptor.go:162-165`). No es un control de función de servicio.
  - La lista blanca `LEDGER_SYSTEM_CREDIT_CLIENT_IDS` es **global**, no por acción.
  - Cedar no tiene tipo `Service` [NO ENCONTRADO].
  - El token de identity es un JWT estático cuyo tipo (usuario o M2M) no se puede determinar desde el código. Si fuera un token de usuario con `customer_id`, podría invocar cualquier ruta AUTHZ.
  - El `x-internal-token` que envía ledger a account no se verifica.
- **Resultado:** **Indeterminado** / correcto por accidente.
- **Falta:**
  - Principal `Service` en Cedar, con políticas por acción (por ejemplo: account solo puede `credit_system` con `reason = registration_bonus`).
  - Clientes M2M por servicio en Cognito, con scopes por operación.
  - Comprobar `client_id` y scope en cada ruta interna.

### E12. Componente de autorización caído → esperado: DENEGAR (fallo seguro)

| Servicio | Comportamiento si AVP falla | Evidencia |
|---|---|---|
| account | `PERMISSION_DENIED avp_error` → **correcto** | `ACC/infra/security/grpc/AuthzServerInterceptor.java:250-262` |
| ledger | `Internal "authorization error"` → **correcto** | `LED/infra/security/grpc/authz_interceptor.go:189-192` |
| identity | No depende de AVP; sus rutas siguen funcionando, pero ninguna es AUTHZ | — |
| ledger `CreditAccountSystem` | No depende de AVP y sigue operativo | — |
| BFF | Lo muestra como 502 | — |

- **Paradoja en account:** una caída de AVP deniega, pero un DENY explícito permite.
- No hay timeout explícito en el cliente AVP (`ACC/infra/config/AwsClientsConfig.java:22-28`).
- **Resultado:** **Correcto** (fallo cerrado), aunque el motivo no se distingue en el cliente.
- **Falta:**
  - Timeouts y circuit breaker hacia AVP.
  - Código de error específico (`AUTHZ_UNAVAILABLE`, 503).
  - Métrica o alarma de `avp_error`.

---

## Tabla resumen

| Esc. | Esperado | Hoy (según el código) | ¿Por diseño? | Motivo trazable | Bloqueante principal |
|---|---|---|---|---|---|
| E1 | Permitir | Permitir* | Sí (código + AVP) | — | *Requiere identity source en AVP |
| E2 | Denegar | **Permitir** | No | No | DENY de AVP no aplicado en account |
| E3 | Permitir | **Denegar** | No | No | Acciones de ledger fuera del esquema |
| E4 | Denegar | Denegar | **Accidente** | No | Ledger no conoce al propietario |
| E5 | Permitir | **Denegar** | No | No | Plantilla `getId()` rota + acciones de ledger |
| E6 | Denegar | Denegar | **Accidente** | No | Nadie comprueba la titularidad del origen |
| E7 | Denegar con motivo | Denegar sin motivo | **Accidente** | No | No hay umbral, `auth_time` ni MFA en las políticas |
| E8 | Denegar con motivo | Denegar sin motivo | **Accidente** | Parcial (`reason` en `ValidateAccountsAndLimits`) | Bug de la plantilla; no hay política de estado |
| E9 | Denegar | Denegar en el borde; micros parcial | Sí / parcial | 401 | Rutas PUBLIC en account; identity con perfil `local` |
| E10 | Denegar | **Mixto** (holds y bono permitidos) | Parcial | No | Rutas internas sin identidad de servicio |
| E11 | Denegar | Indeterminado | No | No | No hay principal Service ni scopes por acción |
| E12 | Denegar | Denegar | Sí | No (502) | — |

**Recuento:**

| Resultado | Escenarios |
|---|---|
| Correcto por diseño | 3: E1\*, E9 en el borde y E12 |
| Correcto por accidente | 4: E4, E6, E7 y E8 |
| Incorrecto | 4: E2, E3, E5 y E10 (parcial) |
| Indeterminado | 1: E11 |

## Conclusión: ¿existe una línea base medible?

**Hoy no existe una línea base ejecutable y reproducible.**

1. **Los casos positivos (E3, E5) fallan.** Los flujos de movimientos y transferencia se rompen antes de llegar a la decisión de autorización: la plantilla `getId()` en account y las acciones de ledger que no están en el esquema. Sin casos positivos que funcionen no se pueden distinguir los verdaderos negativos de los falsos.
2. **El BFF convierte todo en 502**, así que no se puede observar desde el cliente si fue un DENY, un error o una caída.
3. **El entorno no es reproducible.** Faltan los `.env`, el BFF está comentado en compose, los puertos no coinciden, identity usa `local` por defecto y se desconoce el estado real del policy store de AVP.
4. **No hay tests**: solo `contextLoads` en Java y 0 tests en Go.

**Sí existe una línea base analítica**, que es este documento. Para convertirla en **medible** se recomienda definir dos instantáneas.

**Línea base 0 (antes de los controles).** Aplicar solo **correcciones funcionales, no de seguridad**:
- arreglar la plantilla de `ValidateAccountsAndLimits`;
- alinear los IDs de acción de ledger con el esquema, o desactivar AVP de forma explícita y documentada;
- unificar los puertos y destinos, y restaurar el BFF en compose junto con un `.env.example`;
- mapear los errores gRPC a HTTP con `httperr.MapGrpc`;
- usar un perfil explícito distinto de `local` en identity.

Con eso, ejecutar E1–E12 con un script, por ejemplo `grpcurl`/`curl` y Postman/Newman. **Se espera que E2, E4, E6, E7, E10 y E11 se permitan**: esa es la línea base de vulnerabilidad.

**Línea base 1 (después de los controles).** Aplicar los controles de [07](07-pendientes-y-objetivo.md) y repetir la ejecución. Comparar el resultado obtenido con el esperado, el motivo devuelto (`determiningPolicies` o `@id`) y la latencia añadida.
