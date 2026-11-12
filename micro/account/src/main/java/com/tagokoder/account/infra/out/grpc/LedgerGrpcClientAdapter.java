package com.tagokoder.account.infra.out.grpc;

import java.util.UUID;

import org.springframework.stereotype.Component;

import com.tagokoder.account.domain.port.out.LedgerClientPort;
import com.tagokoder.account.infra.config.AppProps;
import com.tagokoder.account.infra.security.grpc.BearerTokenCallCredentials;

import bank.ledgerpayments.v1.LedgerServiceGrpc;
import bank.ledgerpayments.v1.Ledgerpayments.CreditAccountRequest;
import bank.ledgerpayments.v1.Ledgerpayments.CreditAccountResponse;
import bank.ledgerpayments.v1.Ledgerpayments.CreditAccountSystemRequest;
import bank.ledgerpayments.v1.Ledgerpayments.CreditAccountSystemResponse;
import com.tagokoder.account.infra.security.oauth.ServiceTokenProvider;

@Component
public class LedgerGrpcClientAdapter implements LedgerClientPort {

    private final LedgerServiceGrpc.LedgerServiceBlockingStub ledger;
    private final AppProps props;
    private final ServiceTokenProvider serviceTokenProvider;

  public LedgerGrpcClientAdapter(LedgerServiceGrpc.LedgerServiceBlockingStub ledger, AppProps props, ServiceTokenProvider serviceTokenProvider) {
    this.ledger = ledger;
    this.props = props;
    this.serviceTokenProvider = serviceTokenProvider;
  }

    
      @Override
    public String creditAccountSystem(
            String idempotencyKey, UUID accountId, String currency,
            String amount, String externalRef, String reason) {
        CreditAccountSystemRequest req = CreditAccountSystemRequest.newBuilder()
                .setIdempotencyKey(idempotencyKey)
                .setAccountId(accountId.toString())
                .setCurrency(currency)
                .setAmount(amount)
                .setExternalRef(externalRef == null ? "" : externalRef)
                .setReason(reason == null ? "" : reason)
                .build();
        String serviceToken = serviceTokenProvider.getAccessToken();
        // Fase 3: agregar credenciales de servicio verificadas y autorización.
        CreditAccountSystemResponse resp = ledger.withCallCredentials(
            new BearerTokenCallCredentials(serviceToken)
                ).creditAccountSystem(req);
        if (!"posted".equalsIgnoreCase(resp.getStatus()) || resp.getJournalId().isBlank()) {
            throw new IllegalStateException("System credit was not confirmed as posted");
        }
        return resp.getJournalId();
    }
    @Override
    public String creditAccount(
            String idempotencyKey,
            UUID accountId,
            String currency,
            String amount,
            String initiatedBy,
            String externalRef,
            String reason,
            UUID customerId
    ) {
        CreditAccountRequest req = CreditAccountRequest.newBuilder()
                .setIdempotencyKey(idempotencyKey)
                .setAccountId(accountId.toString())
                .setCurrency(currency)
                .setAmount(amount)
                .setInitiatedBy(initiatedBy == null ? "system" : initiatedBy)
                .setExternalRef(externalRef == null ? "" : externalRef)
                .setReason(reason == null ? "" : reason)
                .setCustomerId(customerId == null ? "" : customerId.toString())
                .build();

        CreditAccountResponse resp = ledger.creditAccount(req);
        return resp.getJournalId();
    }
}
