package com.tagokoder.account.infra.out.persistence.jpa.entity;

import java.math.BigDecimal;
import java.time.OffsetDateTime;
import java.util.UUID;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.UpdateTimestamp;
import org.hibernate.annotations.UuidGenerator;
import jakarta.persistence.*;
import lombok.Data;

@Entity
@Table(name = "account_opening_bonus_grants")
@Data
public class OpeningBonusGrantEntity {
    public enum GrantStatus { PENDING, COMPLETED }

    @Id @UuidGenerator
    @Column(name="id", nullable=false, updatable=false)
    private UUID id;
    @Column(name="idempotency_key", nullable=false, length=255)
    private String idempotencyKey;
    @Column(name="customer_id", nullable=false)
    private UUID customerId;
    @Column(name="benefit_code", nullable=false, length=64)
    private String benefitCode;
    @Enumerated(EnumType.STRING)
    @Column(name="status", nullable=false, length=16)
    private GrantStatus status;
    @Column(name="account_id")
    private UUID accountId;
    @Column(name="journal_id")
    private String journalId;
    @Column(name="amount", nullable=false, precision=20, scale=6)
    private BigDecimal amount;
    @Column(name="currency", nullable=false, length=3)
    private String currency;
    @Column(name="locked_at")
    private OffsetDateTime lockedAt;
    @Column(name="locked_by")
    private String lockedBy;
    @CreationTimestamp @Column(name="created_at",nullable=false,updatable=false)
    private OffsetDateTime createdAt;
    @UpdateTimestamp @Column(name="updated_at",nullable=false)
    private OffsetDateTime updatedAt;
}
