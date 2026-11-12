// com.tagokoder.account.infra.in.grpc.validation.AccountsGrpcValidators.java
package com.tagokoder.account.infra.in.grpc.validation;

import com.tagokoder.account.domain.port.in.CreateAccountUseCase;
import com.tagokoder.account.domain.port.in.GetAccountByNumberUseCase;
import com.tagokoder.account.domain.port.in.PatchAccountLimitsUseCase;
import com.tagokoder.account.infra.in.grpc.mapper.ProtoEnumMapper;
import com.tagokoder.account.infra.security.context.AuthCtx;
import bank.accounts.v1.*;

import java.util.UUID;

import static com.tagokoder.account.infra.in.grpc.validation.GrpcValidation.*;

public final class AccountsGrpcValidators {
  private AccountsGrpcValidators() {}

  public static UUID resolveCustomerIdForList(ListAccountsRequest req) {
    var principal = AuthCtx.PRINCIPAL.get();

    if (principal != null && principal.customerIdOrNull() != null && !principal.customerIdOrNull().isBlank()) {
      UUID principalId = requireUuid(principal.customerIdOrNull(), "principal.customer_id");
      if (req.getCustomerId() != null && !req.getCustomerId().isBlank()) {
        UUID requested = requireUuid(req.getCustomerId(), "customer_id");
        if (!requested.equals(principalId)) throw forbidden("Forbidden");
      }
      return principalId;
    }
    return requireUuid(req.getCustomerId(), "customer_id");
  }

  public static GetAccountByNumberUseCase.Command toGetByNumberCommand(GetAccountByNumberRequest req) {
    String acct = requireAccountNumber12(req.getAccountNumber(), "account_number");
    boolean includeInactive = req.hasIncludeInactive() && req.getIncludeInactive().getValue();
    return new GetAccountByNumberUseCase.Command(acct, includeInactive);
  }

  /** PUBLIC CreateAccount: self-only + NO bono */
  public static CreateAccountUseCase.Command toCreateAccountCommand(CreateAccountRequest req) {
    UUID customerId = requireUuid(req.getCustomerId(), "customer_id");
    String productType = ProtoEnumMapper.mapProductType(req.getProductType());
    String currency = requireCurrency(req.getCurrency());

    // idempotency_key: requerido (mutación)
    requireIdempotencyKey(req.getIdempotencyKey(), "idempotency_key");

    // external_ref opcional (normalizado)
    optionalExternalRef(req.getExternalRef(), "external_ref");

    // self-only si hay principal (usuario)
    var principal = AuthCtx.PRINCIPAL.get();
    if (principal != null && principal.customerIdOrNull() != null && !principal.customerIdOrNull().isBlank()) {
      UUID principalId = requireUuid(principal.customerIdOrNull(), "principal.customer_id");
      if (!principalId.equals(customerId)) throw forbidden("Forbidden");
    }

    return new CreateAccountUseCase.Command(customerId, productType, currency);
  }

  public static UUID toAccountId(GetAccountBalancesRequest req) {
    return requireUuid(req.getId(), "id");
  }

  public record PatchLimitsIn(String idempotencyKey, PatchAccountLimitsUseCase.Command command) {}

  public static PatchLimitsIn toPatchLimitsInput(PatchAccountLimitsRequest req, String idempotencyKeyFromCtx) {
    String key = requireIdempotencyKey(idempotencyKeyFromCtx, "idempotency-key");
    UUID accountId = requireUuid(req.getId(), "id");

    boolean hasAny = req.hasDailyOut() || req.hasDailyIn();
    if (!hasAny) throw invalid("At least one of daily_out or daily_in is required");

    var dailyOut = optionalNonNegativeMoney(req.hasDailyOut() ? req.getDailyOut().getValue() : null, "daily_out");
    var dailyIn  = optionalNonNegativeMoney(req.hasDailyIn()  ? req.getDailyIn() .getValue() : null, "daily_in");

    return new PatchLimitsIn(key, new PatchAccountLimitsUseCase.Command(accountId, dailyOut, dailyIn));
  }

  public static void enforceSelfOnlyCustomer(UUID ownerCustomerId) {
    var principal = AuthCtx.PRINCIPAL.get();
    if (principal == null) return;

    String cid = principal.customerIdOrNull();
    if (cid == null || cid.isBlank()) return;

    UUID principalId = requireUuid(cid, "principal.customer_id");
    if (!principalId.equals(ownerCustomerId)) throw forbidden("Forbidden");
  }
}