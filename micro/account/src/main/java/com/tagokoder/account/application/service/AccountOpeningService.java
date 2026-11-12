package com.tagokoder.account.application.service;

import java.math.BigDecimal;
import java.util.UUID;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

import com.tagokoder.account.application.AccountNumberFmt;
import com.tagokoder.account.domain.port.in.CreateAccountUseCase;
import com.tagokoder.account.domain.port.in.OpenAccountWithOpeningBonusUseCase;
import com.tagokoder.account.domain.port.out.AccountRepositoryPort;
import com.tagokoder.account.domain.port.out.LedgerClientPort;
import com.tagokoder.account.domain.port.out.OpeningBonusRepositoryPort;

@Service
public class AccountOpeningService implements OpenAccountWithOpeningBonusUseCase {

    private static final BigDecimal OPENING_BONUS = new BigDecimal("50.00");
    private static final String BONUS_REASON = "registration_bonus";
    private static final String BONUS_EXTERNAL_REF = "bonus:registration";

    private final CreateAccountUseCase createAccount;
    private final LedgerClientPort ledger;
    private final OpeningBonusRepositoryPort openingBonusRepo;
    private final AccountRepositoryPort accountRepo;

    public AccountOpeningService(
            CreateAccountUseCase createAccount,
            LedgerClientPort ledger,
            OpeningBonusRepositoryPort openingBonusRepo,
            AccountRepositoryPort accountRepo
    ) {
        this.createAccount = createAccount;
        this.ledger = ledger;
        this.openingBonusRepo = openingBonusRepo;
        this.accountRepo = accountRepo;
    }
    @Override
    @Transactional(propagation = Propagation.NOT_SUPPORTED)
    public Result open(Command c) {

    UUID customerId = c.customerId();
    String currency = c.currency();

    String baseKey = normalizeKey(c.idempotencyKey(), customerId, c.productType(), currency);
    String bonusKey = baseKey + ":opening_bonus";
    // Una promoción por customerId, aunque lleguen claves distintas.
    var grantByCustomer = openingBonusRepo.findByCustomerAndBenefit(
        customerId, OpeningBonusRepositoryPort.REGISTRATION_BONUS);
    if (grantByCustomer.isPresent()) {
        bonusKey = grantByCustomer.get().idempotencyKey();
    }

    // 0) Leer estado actual
    var ex = openingBonusRepo.findByKey(bonusKey);

    if (ex.isPresent() && ex.get().status() == OpeningBonusRepositoryPort.Status.COMPLETED) {
        var g = ex.get();
        String accNum = accountRepo.findById(g.accountId())
            .map(a -> AccountNumberFmt.fmt12(a.getAccountNumber()))
            .orElse("");
        return new Result(g.accountId(), accNum, g.journalId(), "opened");
    }

    // 1) Si no existe, intentar adquirir (insert PENDING)
    if (ex.isEmpty()) {
        openingBonusRepo.tryAcquire(bonusKey, customerId,
            OpeningBonusRepositoryPort.REGISTRATION_BONUS, OPENING_BONUS, currency);
        ex = openingBonusRepo.findByCustomerAndBenefit(customerId,
            OpeningBonusRepositoryPort.REGISTRATION_BONUS);
        if (ex.isPresent()) bonusKey = ex.get().idempotencyKey();
    }

    var now = ex.orElseThrow(() -> new IllegalStateException("opening bonus grant missing"));
    if (!customerId.equals(now.customerId()) || !"USD".equals(currency)
            || now.amount().compareTo(OPENING_BONUS) != 0) {
        throw new IllegalStateException("opening bonus grant conflicts with request");
    }

    // 2) Determinar accountId
    UUID accountId = now.accountId();
    String accountNumber;

    if (accountId == null) {
        // este request crea la cuenta y la “ancla” al grant
        var created = createAccount.create(new CreateAccountUseCase.Command(
            customerId, c.productType(), currency
        ));
        accountId = created.accountId();
        accountNumber = AccountNumberFmt.fmt12(created.accountNumber());

        // importante: anclar account_id para reintentos (evita doble cuenta si falla después)
        if (!openingBonusRepo.attachAccountIfEmpty(bonusKey, accountId)) {
            var existing = openingBonusRepo.findByKey(bonusKey).orElseThrow();
            if (!accountId.equals(existing.accountId())) {
                throw new IllegalStateException("Concurrent opening conflict; retry with same key");
            }
        }
    } else {
        accountNumber = accountRepo.findById(accountId)
            .map(a -> AccountNumberFmt.fmt12(a.getAccountNumber()))
            .orElse("");
    }

    // 3) Postear bono en Ledger (idempotente por bonusKey)
    String journalId = ledger.creditAccountSystem(
        bonusKey,
        accountId,
        currency,
        OPENING_BONUS.toPlainString(),
        c.externalRef(),
        BONUS_REASON
    );

    // 4) Completar PENDING→COMPLETED (solo 1 gana)
    boolean transitioned = openingBonusRepo.completeIfPending(bonusKey, accountId, journalId);
    if (!transitioned) {
        var grant = openingBonusRepo.findByKey(bonusKey).orElseThrow();
        if (grant.status() != OpeningBonusRepositoryPort.Status.COMPLETED
                || !accountId.equals(grant.accountId())
                || !journalId.equals(grant.journalId())) {
            throw new IllegalStateException("Bonus completion conflict");
        }
    }
    // El saldo se actualizara solo con eventos contables en la Fase 4.

    return new Result(accountId, accountNumber, journalId, "opened");
    }

    private static String normalizeKey(String key, UUID customerId, String productType, String currency) {
        if (key != null && !key.isBlank()) return key.trim();
        String pt = (productType == null) ? "unknown" : productType;
        String cur = (currency == null) ? "XXX" : currency;
        return "open:" + customerId + ":" + pt + ":" + cur;
    }
}
