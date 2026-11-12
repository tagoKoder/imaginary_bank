package com.tagokoder.account.infra.out.persistence.jpa;

import java.util.Optional;
import java.util.UUID;
import org.springframework.data.jpa.repository.*;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;
import com.tagokoder.account.infra.out.persistence.jpa.entity.OpeningBonusGrantEntity;

@Repository
public interface SpringDataOpeningBonusGrantJpa extends JpaRepository<OpeningBonusGrantEntity, UUID> {
    Optional<OpeningBonusGrantEntity> findByIdempotencyKey(String key);
    Optional<OpeningBonusGrantEntity> findByCustomerIdAndBenefitCode(UUID customerId, String benefitCode);

    @Modifying
    @Query(value = """
       UPDATE account_opening_bonus_grants g
       SET account_id = :accountId, updated_at = now()
       WHERE g.idempotency_key = :key AND g.status = 'PENDING' AND g.account_id IS NULL
       AND EXISTS (SELECT 1 FROM accounts a
                   WHERE a.id = :accountId AND a.customer_id = g.customer_id)
       """, nativeQuery = true)
    int attachAccountIfEmpty(@Param("key") String key, @Param("accountId") UUID accountId);

    @Modifying
    @Query(value = """
       UPDATE account_opening_bonus_grants g
       SET status = 'COMPLETED', journal_id = :journalId,
           locked_at = NULL, locked_by = NULL, updated_at = now()
       WHERE g.idempotency_key = :key AND g.status = 'PENDING'
         AND g.account_id = :accountId AND g.journal_id IS NULL
       """, nativeQuery = true)
    int completeIfPending(@Param("key") String key,
        @Param("accountId") UUID accountId, @Param("journalId") String journalId);
}
