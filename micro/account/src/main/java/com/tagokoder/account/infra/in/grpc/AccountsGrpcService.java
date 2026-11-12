package com.tagokoder.account.infra.in.grpc;

import static com.tagokoder.account.infra.in.grpc.validation.AccountsGrpcValidators.resolveCustomerIdForList;
import static com.tagokoder.account.infra.in.grpc.validation.AccountsGrpcValidators.toAccountId;
import static com.tagokoder.account.infra.in.grpc.validation.AccountsGrpcValidators.toCreateAccountCommand;
import static com.tagokoder.account.infra.in.grpc.validation.AccountsGrpcValidators.toGetByNumberCommand;
import static com.tagokoder.account.infra.in.grpc.validation.AccountsGrpcValidators.toPatchLimitsInput;

import java.time.OffsetDateTime;
import java.util.UUID;

import com.google.protobuf.Timestamp;
import com.tagokoder.account.application.AccountNumberFmt;
import com.tagokoder.account.application.service.IdempotencyService;
import com.tagokoder.account.domain.port.in.CreateAccountUseCase;
import com.tagokoder.account.domain.port.in.GetAccountBalancesUseCase;
import com.tagokoder.account.domain.port.in.GetAccountByNumberUseCase;
import com.tagokoder.account.domain.port.in.ListAccountsUseCase;
import com.tagokoder.account.domain.port.in.PatchAccountLimitsUseCase;
import com.tagokoder.account.infra.in.grpc.mapper.ProtoEnumMapper;
import com.tagokoder.account.infra.security.grpc.IdempotencyKeyInterceptor;

import bank.accounts.v1.AccountBalances;
import bank.accounts.v1.AccountLookup;
import bank.accounts.v1.AccountView;
import bank.accounts.v1.AccountsServiceGrpc;
import bank.accounts.v1.CreateAccountRequest;
import bank.accounts.v1.CreateAccountResponse;
import bank.accounts.v1.GetAccountBalancesRequest;
import bank.accounts.v1.GetAccountBalancesResponse;
import bank.accounts.v1.GetAccountByNumberRequest;
import bank.accounts.v1.GetAccountByNumberResponse;
import bank.accounts.v1.ListAccountsRequest;
import bank.accounts.v1.ListAccountsResponse;
import bank.accounts.v1.PatchAccountLimitsRequest;
import bank.accounts.v1.PatchAccountLimitsResponse;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;
@GrpcService
public class AccountsGrpcService extends AccountsServiceGrpc.AccountsServiceImplBase {

    private final ListAccountsUseCase listAccounts;
    private final GetAccountByNumberUseCase getByNumber;
    private final GetAccountBalancesUseCase getBalances;
    private final PatchAccountLimitsUseCase patchLimits;
    private final IdempotencyService idem;
    private final CreateAccountUseCase createAccount; 

    public AccountsGrpcService(
            ListAccountsUseCase listAccounts,
            GetAccountByNumberUseCase getByNumber,
            GetAccountBalancesUseCase getBalances,
            PatchAccountLimitsUseCase patchLimits,
            IdempotencyService idem,
            CreateAccountUseCase createAccount
    ) {
        this.listAccounts = listAccounts;
        this.getByNumber = getByNumber;
        this.getBalances = getBalances;
        this.patchLimits = patchLimits;
        this.idem = idem;
        this.createAccount= createAccount;
    }

    @Override
    public void listAccounts(ListAccountsRequest request, StreamObserver<ListAccountsResponse> responseObserver) {
        UUID customerId = resolveCustomerIdForList(request);
        var res = listAccounts.listByCustomer(customerId);

        System.out.println("AUTHZ request customerId={}"+ customerId);

        ListAccountsResponse.Builder out = ListAccountsResponse.newBuilder();
        res.accounts().forEach(a -> {
        AccountView.Builder view = AccountView.newBuilder()
        .setId(a.id().toString())
        .setCustomerId(a.customerId().toString())
        .setProductType(a.productType())
        .setCurrency(a.currency())
        .setStatus(a.status())
        .setOpenedAt(toTimestamp(a.openedAt()))
        .setUpdatedAt(toTimestamp(a.updatedAt()))
        .setBalances(AccountBalances.newBuilder()
                .setLedger(GrpcMoney.str(a.balances().ledger()))
                .setAvailable(GrpcMoney.str(a.balances().available()))
                .setHold(GrpcMoney.str(a.balances().hold()))
                .build());
        out.addAccounts(view.build());
        });

        responseObserver.onNext(out.build());
        responseObserver.onCompleted();
    }


    @Override
    public void getAccountByNumber(GetAccountByNumberRequest request,
                                StreamObserver<GetAccountByNumberResponse> responseObserver) {
        var cmd = toGetByNumberCommand(request);
        var res = getByNumber.getByNumber(cmd);

        var a = res.account();

        responseObserver.onNext(GetAccountByNumberResponse.newBuilder()
                .setAccount(AccountLookup.newBuilder()
                        .setAccountId(a.accountId().toString())
                        .setCustomerId(a.customerId().toString())
                        .setAccountNumber(a.accountNumber())
                        .setDisplayName(a.displayName())
                        .setCurrency(a.currency())
                        .setStatus(a.status())
                        .setProductType(ProtoEnumMapper.mapProductTypeToEnum(a.productType()))
                        .build())
                .build());

        responseObserver.onCompleted();
    }

    @Override
    public void createAccount(CreateAccountRequest request, StreamObserver<CreateAccountResponse> responseObserver) {
        String idemKey = request.getIdempotencyKey();
        // cache idempotente (opcional pero recomendado)
        var cached = idem.tryGet(idemKey, "accounts.createAccount", CreateAccountUseCase.Result.class);
        if (cached.isPresent()) {
        var r = cached.get();
        responseObserver.onNext(CreateAccountResponse.newBuilder()
            .setAccountId(r.accountId().toString())
            .setAccountNumber(AccountNumberFmt.fmt12(r.accountNumber()))
            .build());
        responseObserver.onCompleted();
        return;
        }

        var cmd = toCreateAccountCommand(request);
        var res = createAccount.create(cmd);

        idem.save(idemKey, "accounts.createAccount", 200, res);

        responseObserver.onNext(CreateAccountResponse.newBuilder()
            .setAccountId(res.accountId().toString())
            .setAccountNumber(AccountNumberFmt.fmt12(res.accountNumber()))
            .build());
        responseObserver.onCompleted();
    }

        @Override
        public void getAccountBalances(GetAccountBalancesRequest request, StreamObserver<GetAccountBalancesResponse> responseObserver) {
            UUID accountId = toAccountId(request);
            var res = getBalances.get(accountId);

            responseObserver.onNext(GetAccountBalancesResponse.newBuilder()
            .setAccountId(res.accountId().toString())
            .setBalances(AccountBalances.newBuilder()
            .setLedger(GrpcMoney.str(res.ledger()))
            .setAvailable(GrpcMoney.str(res.available()))
            .setHold(GrpcMoney.str(res.hold()))
            .build())
            .build());
            responseObserver.onCompleted();
        }

        @Override
        public void patchAccountLimits(PatchAccountLimitsRequest request, StreamObserver<PatchAccountLimitsResponse> responseObserver) {
                var in = toPatchLimitsInput(request, IdempotencyKeyInterceptor.IDEMPOTENCY_KEY.get());

                // 1) si ya existe, responde lo mismo sin re-ejecutar
                var cached = idem.tryGet(in.idempotencyKey(), "accounts.patchLimits", PatchAccountLimitsUseCase.Result.class);
                if (cached.isPresent()) {
                var res = cached.get();
                responseObserver.onNext(PatchAccountLimitsResponse.newBuilder()
                .setAccountId(res.accountId().toString())
                .setDailyOut(GrpcMoney.str(res.dailyOut()))
                .setDailyIn(GrpcMoney.str(res.dailyIn()))
                .build());
                responseObserver.onCompleted();
                return;
                }

                // 2) ejecuta caso normal
                var res = patchLimits.patch(in.command());

                // 3) guarda respuesta idempotente
                idem.save(in.idempotencyKey(), "accounts.patchLimits", 200, res);

                responseObserver.onNext(PatchAccountLimitsResponse.newBuilder()
                .setAccountId(res.accountId().toString())
                .setDailyOut(GrpcMoney.str(res.dailyOut()))
                .setDailyIn(GrpcMoney.str(res.dailyIn()))
                .build());
                responseObserver.onCompleted();
        }

    private static Timestamp toTimestamp(OffsetDateTime i) {
        if (i == null) return Timestamp.getDefaultInstance();
        return Timestamp.newBuilder()
                .setSeconds(i.toEpochSecond())
                .setNanos(i.getNano())
                .build();
    }
}
