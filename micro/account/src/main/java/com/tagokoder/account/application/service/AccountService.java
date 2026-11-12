package com.tagokoder.account.application.service;

import java.math.BigDecimal;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.UUID;
import java.util.stream.Collectors;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import com.tagokoder.account.application.AccountNumberFmt;
import com.tagokoder.account.domain.model.Account;
import com.tagokoder.account.domain.port.in.BatchGetAccountSummariesUseCase;
import com.tagokoder.account.domain.port.in.CreateAccountUseCase;
import com.tagokoder.account.domain.port.in.GetAccountBalancesUseCase;
import com.tagokoder.account.domain.port.in.GetAccountByNumberUseCase;
import com.tagokoder.account.domain.port.in.ListAccountsUseCase;
import com.tagokoder.account.domain.port.in.PatchAccountLimitsUseCase;
import com.tagokoder.account.domain.port.in.ReleaseHoldUseCase;
import com.tagokoder.account.domain.port.in.ReserveHoldUseCase;
import com.tagokoder.account.domain.port.in.ValidateAccountsAndLimitsUseCase;
import com.tagokoder.account.domain.port.out.AccountBalanceRepositoryPort;
import com.tagokoder.account.domain.port.out.AccountLimitsRepositoryPort;
import com.tagokoder.account.domain.port.out.AccountRepositoryPort;
import com.tagokoder.account.domain.port.out.CustomerRepositoryPort;

@Service
public class AccountService implements
        CreateAccountUseCase,
        ListAccountsUseCase,
        GetAccountBalancesUseCase,
        PatchAccountLimitsUseCase,
        ValidateAccountsAndLimitsUseCase,
        ReserveHoldUseCase,
        ReleaseHoldUseCase,
        GetAccountByNumberUseCase,
        BatchGetAccountSummariesUseCase {

    private final AccountRepositoryPort accountRepo;
    private final AccountBalanceRepositoryPort balanceRepo;
    private final AccountLimitsRepositoryPort limitsRepo;
    private final CustomerRepositoryPort customerRepo;

    public AccountService(AccountRepositoryPort accountRepo,
                          AccountBalanceRepositoryPort balanceRepo,
                          AccountLimitsRepositoryPort limitsRepo,
                          CustomerRepositoryPort customerRepo) {
        this.accountRepo = accountRepo;
        this.balanceRepo = balanceRepo;
        this.limitsRepo = limitsRepo;
        this.customerRepo = customerRepo;
    }

    @Override
    @Transactional
    public CreateAccountUseCase.Result create(CreateAccountUseCase.Command c) {
        if (c == null || c.customerId() == null) throw new IllegalArgumentException("customerId required");
        if (!"USD".equalsIgnoreCase(c.currency())) throw new IllegalArgumentException("Only USD supported for now");
        if (!"checking".equalsIgnoreCase(c.productType())
                && !"savings".equalsIgnoreCase(c.productType())) {
            throw new IllegalArgumentException("Unsupported product type");
        }
        if (!accountRepo.existsCustomer(c.customerId())) {
            throw new IllegalArgumentException("Customer not found");
        }
        var existing = accountRepo.findByCustomerId(c.customerId());
        if (!existing.isEmpty()) {
            Account account = existing.get(0);
            if (!c.currency().equalsIgnoreCase(account.getCurrency()) ||
                !c.productType().equalsIgnoreCase(account.getProductType())) {
                throw new IllegalStateException("Customer already has a different account");
            }
            return new CreateAccountUseCase.Result(account.getId(), account.getAccountNumber());
        }

        Account a = new Account();
        a.setCustomerId(c.customerId());
        a.setProductType(c.productType());
        a.setCurrency(c.currency());
        a.setStatus("active");
        a.setOpenedAt(Instant.now());
        a.setUpdatedAt(Instant.now());
        
        Account saved = accountRepo.save(a);
        //BigDecimal bonus = new BigDecimal("50.00");
        balanceRepo.init(saved.getId(), BigDecimal.ZERO, BigDecimal.ZERO, BigDecimal.ZERO);
        //balanceRepo.initZero(saved.getId());
        
        limitsRepo.patch(saved.getId(), BigDecimal.ZERO, BigDecimal.ZERO);

        return new CreateAccountUseCase.Result(saved.getId(), saved.getAccountNumber());
    }

    @Override
    @Transactional(readOnly = true)
    public GetAccountByNumberUseCase.Result getByNumber(GetAccountByNumberUseCase.Command c) {
        if (c == null || c.accountNumber() == null || c.accountNumber().isBlank()) {
            throw new IllegalArgumentException("accountNumber is required");
        }

        String raw = c.accountNumber().trim();
        if (!raw.matches("^\\d{1,12}$")) {
            throw new IllegalArgumentException("accountNumber must be numeric up to 12 digits");
        }

        long n = Long.parseLong(raw);

        var acc = accountRepo.findByAccountNumber(n)
                .orElseThrow(() -> new IllegalArgumentException("account not found"));

        if (!c.includeInactive() && !"active".equalsIgnoreCase(acc.getStatus())) {
            throw new IllegalArgumentException("account is inactive");
        }

        String displayName = customerRepo.findFullNamesByIds(java.util.List.of(acc.getCustomerId()))
                .getOrDefault(acc.getCustomerId(), "");

        var view = new GetAccountByNumberUseCase.AccountLookupView(
                acc.getId(),
                acc.getCustomerId(),
                AccountNumberFmt.fmt12(acc.getAccountNumber()),
                displayName,
                safe(acc.getCurrency()),
                safe(acc.getStatus()),
                safe(acc.getProductType())
        );

        return new GetAccountByNumberUseCase.Result(view);
    }

    @Override
    public ListAccountsUseCase.Result listByCustomer(UUID customerId) {
        List<Account> accounts = accountRepo.findByCustomerId(customerId);

        var views = accounts.stream().map(a -> {
            var bal = balanceRepo.findByAccountId(a.getId())
                    .orElse(new AccountBalanceRepositoryPort.BalancesRow(BigDecimal.ZERO, BigDecimal.ZERO, BigDecimal.ZERO));

            return new ListAccountsUseCase.AccountView(
                    a.getId(),
                    a.getCustomerId(),
                    AccountNumberFmt.fmt12(a.getAccountNumber()),
                    a.getProductType(),
                    a.getCurrency(),
                    a.getStatus(),
                    a.getOpenedAt().atOffset(ZoneOffset.UTC),
                    a.getUpdatedAt().atOffset(ZoneOffset.UTC),
                    new ListAccountsUseCase.Balances(bal.ledger(), bal.available(), bal.hold())
            );
        }).toList();

        return new ListAccountsUseCase.Result(views);
    }

    @Override
    public GetAccountBalancesUseCase.Result get(UUID accountId) {
        var bal = balanceRepo.findByAccountId(accountId)
                .orElseThrow(() -> new IllegalArgumentException("Account balances not found"));
        return new GetAccountBalancesUseCase.Result(accountId, bal.ledger(), bal.available(), bal.hold());
    }

    @Override
    @Transactional
    public PatchAccountLimitsUseCase.Result patch(PatchAccountLimitsUseCase.Command c) {
        if (c.dailyOut() != null && c.dailyOut().compareTo(BigDecimal.ZERO) < 0) throw new IllegalArgumentException("dailyOut must be >= 0");
        if (c.dailyIn() != null && c.dailyIn().compareTo(BigDecimal.ZERO) < 0) throw new IllegalArgumentException("dailyIn must be >= 0");

        var row = limitsRepo.patch(c.accountId(), c.dailyOut(), c.dailyIn());
        return new PatchAccountLimitsUseCase.Result(c.accountId(), row.dailyOut(), row.dailyIn());
    }

    
    @Override
    @Transactional(readOnly = true)
    public ValidateAccountsAndLimitsUseCase.Result validate(ValidateAccountsAndLimitsUseCase.Command c) {
        if (c.amount() == null || c.amount().compareTo(BigDecimal.ZERO) <= 0) {
            return new ValidateAccountsAndLimitsUseCase.Result(false, "amount must be > 0");
        }
        if (c.currency() == null || c.currency().length() != 3) {
            return new ValidateAccountsAndLimitsUseCase.Result(false, "invalid currency");
        }

        var srcOpt = accountRepo.findById(c.sourceAccountId());
        if (srcOpt.isEmpty()) return new ValidateAccountsAndLimitsUseCase.Result(false, "source account not found");

        var dstOpt = accountRepo.findById(c.destinationAccountId());
        if (dstOpt.isEmpty()) return new ValidateAccountsAndLimitsUseCase.Result(false, "destination account not found");

        var src = srcOpt.get();
        var dst = dstOpt.get();

        if (!"active".equalsIgnoreCase(src.getStatus())) return new ValidateAccountsAndLimitsUseCase.Result(false, "source not active");
        if (!"active".equalsIgnoreCase(dst.getStatus())) return new ValidateAccountsAndLimitsUseCase.Result(false, "destination not active");

        if (!c.currency().equalsIgnoreCase(src.getCurrency())) return new ValidateAccountsAndLimitsUseCase.Result(false, "source currency mismatch");
        if (!c.currency().equalsIgnoreCase(dst.getCurrency())) return new ValidateAccountsAndLimitsUseCase.Result(false, "destination currency mismatch");

        boolean isCredit = c.sourceAccountId().equals(c.destinationAccountId());

        if (isCredit) {
            // CREDIT: NO validar available.
            // (opcional) validar dailyIn sobre DESTINO
            var limOpt = limitsRepo.findByAccountId(dst.getId());
            if (limOpt.isPresent()) {
            var lim = limOpt.get();
            if (lim.dailyIn().compareTo(BigDecimal.ZERO) > 0 && c.amount().compareTo(lim.dailyIn()) > 0) {
                return new ValidateAccountsAndLimitsUseCase.Result(false, "dailyIn limit exceeded");
            }
            }
            return new ValidateAccountsAndLimitsUseCase.Result(true, null);
        }

        // DEBIT/TRANSFER: validar available y dailyOut (como ya tienes)
        var bal = balanceRepo.findByAccountId(src.getId())
            .orElse(new AccountBalanceRepositoryPort.BalancesRow(BigDecimal.ZERO, BigDecimal.ZERO, BigDecimal.ZERO));

        if (bal.available().compareTo(c.amount()) < 0) {
            return new ValidateAccountsAndLimitsUseCase.Result(false, "insufficient available");
        }

        var limOpt = limitsRepo.findByAccountId(src.getId());
        if (limOpt.isPresent()) {
            var lim = limOpt.get();
            if (lim.dailyOut().compareTo(BigDecimal.ZERO) > 0 && c.amount().compareTo(lim.dailyOut()) > 0) {
            return new ValidateAccountsAndLimitsUseCase.Result(false, "dailyOut limit exceeded");
            }
        }

        return new ValidateAccountsAndLimitsUseCase.Result(true, null);
    }


    @Override
    @Transactional
    public ReserveHoldUseCase.Result reserve(ReserveHoldUseCase.Command c) {
        if (c.amount() == null || c.amount().compareTo(BigDecimal.ZERO) <= 0) throw new IllegalArgumentException("amount must be > 0");
        var acc = accountRepo.findById(c.accountId()).orElseThrow(() -> new IllegalArgumentException("account not found"));

        if (!"active".equalsIgnoreCase(acc.getStatus())) throw new IllegalArgumentException("account not active");
        if (!c.currency().equalsIgnoreCase(acc.getCurrency())) throw new IllegalArgumentException("currency mismatch");

        // ATÓMICO: incrementa hold y valida que available sigue suficiente
        var newHold = balanceRepo.incrementHold(c.accountId(), c.amount());

        return new ReserveHoldUseCase.Result(true, newHold);
    }

    @Override
    @Transactional
    public ReleaseHoldUseCase.Result release(ReleaseHoldUseCase.Command c) {
        if (c.amount() == null || c.amount().compareTo(BigDecimal.ZERO) <= 0) throw new IllegalArgumentException("amount must be > 0");
        var acc = accountRepo.findById(c.accountId()).orElseThrow(() -> new IllegalArgumentException("account not found"));

        if (!c.currency().equalsIgnoreCase(acc.getCurrency())) throw new IllegalArgumentException("currency mismatch");

        var newHold = balanceRepo.decrementHold(c.accountId(), c.amount());
        return new ReleaseHoldUseCase.Result(true, newHold);
    }

    @Override
    @Transactional(readOnly = true)
    public BatchGetAccountSummariesUseCase.Result batchGetSummaries(BatchGetAccountSummariesUseCase.Command c) {
        List<UUID> requested = (c.accountIds() == null) ? List.of() : c.accountIds();

        // dedup pero preservando orden
        LinkedHashSet<UUID> orderedSet = new LinkedHashSet<>(requested);
        List<UUID> ids = new ArrayList<>(orderedSet);

        if (ids.isEmpty()) {
            return new BatchGetAccountSummariesUseCase.Result(List.of(), List.of());
        }

        // batch load accounts
        List<Account> accounts = accountRepo.findByIds(ids);
        Map<UUID, Account> byId = accounts.stream()
                .collect(Collectors.toMap(Account::getId, a -> a, (a, b) -> a));

        // batch load customer names
        List<UUID> customerIds = accounts.stream()
                .map(Account::getCustomerId)
                .filter(Objects::nonNull)
                .distinct()
                .toList();

        Map<UUID, String> namesByCustomerId = customerRepo.findFullNamesByIds(customerIds);

        boolean includeInactive = c.includeInactive();

        List<BatchGetAccountSummariesUseCase.AccountSummaryView> ok = new ArrayList<>();
        List<BatchGetAccountSummariesUseCase.Missing> missing = new ArrayList<>();

        for (UUID id : ids) {
            Account a = byId.get(id);
            if (a == null) {
                missing.add(new BatchGetAccountSummariesUseCase.Missing(id, "not_found"));
                continue;
            }

            String status = safe(a.getStatus());
            if (!includeInactive && !"active".equalsIgnoreCase(status)) {
                missing.add(new BatchGetAccountSummariesUseCase.Missing(id, "inactive"));
                continue;
            }

            String displayName = namesByCustomerId.getOrDefault(a.getCustomerId(), "");
            ok.add(new BatchGetAccountSummariesUseCase.AccountSummaryView(
                    a.getId(),
                    AccountNumberFmt.fmt12(a.getAccountNumber()),
                    safe(a.getProductType()),
                    safe(a.getCurrency()),
                    status,
                    displayName
            ));
        }

        return new BatchGetAccountSummariesUseCase.Result(ok, missing);
    }


    private static String safe(String s) {
        return s == null ? "" : s;
    }
}
