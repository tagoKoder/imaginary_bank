package com.tagokoder.account.infra.out.persistence.jpa.adapter;

import java.math.BigDecimal;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;
import com.tagokoder.account.domain.port.out.OpeningBonusRepositoryPort;
import com.tagokoder.account.infra.out.persistence.jpa.SpringDataOpeningBonusGrantJpa;
import com.tagokoder.account.infra.out.persistence.jpa.entity.OpeningBonusGrantEntity;

@Component
public class OpeningBonusRepositoryAdapter implements OpeningBonusRepositoryPort {
    private final SpringDataOpeningBonusGrantJpa jpa;
    private final JdbcTemplate jdbc;

    public OpeningBonusRepositoryAdapter(SpringDataOpeningBonusGrantJpa jpa, JdbcTemplate jdbc) {
        this.jpa = jpa;
        this.jdbc = jdbc;
    }

    private BonusGrant map(OpeningBonusGrantEntity e) {
        return new BonusGrant(e.getIdempotencyKey(), e.getCustomerId(), e.getBenefitCode(),
            Status.valueOf(e.getStatus().name()), e.getAccountId(), e.getJournalId(),
            e.getAmount(), e.getCurrency());
    }

    @Override
    public Optional<BonusGrant> findByKey(String key) {
        return jpa.findByIdempotencyKey(key).map(this::map);
    }

    @Override
    public Optional<BonusGrant> findByCustomerAndBenefit(UUID customerId, String benefitCode) {
        return jpa.findByCustomerIdAndBenefitCode(customerId, benefitCode).map(this::map);
    }

    @Override
    @Transactional
    public boolean tryAcquire(String key, UUID customerId, String benefitCode,
                              BigDecimal amount, String currency) {
        if (key == null || key.isBlank() || customerId == null || benefitCode == null
                || amount == null || currency == null) {
            throw new IllegalArgumentException("incomplete bonus acquisition request");
        }
        int inserted = jdbc.update("""
            INSERT INTO account_opening_bonus_grants
              (id, idempotency_key, customer_id, benefit_code, status, amount,
               currency, locked_at, locked_by, created_at, updated_at)
            VALUES (?, ?, ?, ?, 'PENDING', ?, ?, now(), 'svc:account', now(), now())
            ON CONFLICT DO NOTHING
            """, UUID.randomUUID(), key, customerId, benefitCode, amount, currency);
        return inserted == 1;
    }

    @Override
    @Transactional
    public boolean attachAccountIfEmpty(String key, UUID accountId) {
        return jpa.attachAccountIfEmpty(key, accountId) == 1;
    }

    @Override
    @Transactional
    public boolean completeIfPending(String key, UUID accountId, String journalId) {
        return jpa.completeIfPending(key, accountId, journalId) == 1;
    }
}
