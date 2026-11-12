package com.tagokoder.account.domain.port.out;

import java.math.BigDecimal;
import java.util.Optional;
import java.util.UUID;

public interface OpeningBonusRepositoryPort {
    String REGISTRATION_BONUS = "REGISTRATION_BONUS";
    enum Status { PENDING, COMPLETED }

    record BonusGrant(
        String idempotencyKey,
        UUID customerId,
        String benefitCode,
        Status status,
        UUID accountId,
        String journalId,
        BigDecimal amount,
        String currency
    ) {}

    Optional<BonusGrant> findByKey(String key);
    Optional<BonusGrant> findByCustomerAndBenefit(UUID customerId, String benefitCode);

    // INSERT ... ON CONFLICT DO NOTHING: no deja transacción Postgres abortada.
    boolean tryAcquire(String key, UUID customerId, String benefitCode,
                       BigDecimal amount, String currency);
    boolean attachAccountIfEmpty(String key, UUID accountId);
    boolean completeIfPending(String key, UUID accountId, String journalId);
}
