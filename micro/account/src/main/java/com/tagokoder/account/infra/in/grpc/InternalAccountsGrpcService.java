package com.tagokoder.account.infra.in.grpc;

import static com.tagokoder.account.infra.in.grpc.validation.InternalAccountsGrpcValidators.toBatchIdsInput;
import static com.tagokoder.account.infra.in.grpc.validation.InternalAccountsGrpcValidators.toOpenAccountWithBonusCommand;
import static com.tagokoder.account.infra.in.grpc.validation.InternalAccountsGrpcValidators.toReleaseCommand;
import static com.tagokoder.account.infra.in.grpc.validation.InternalAccountsGrpcValidators.toReleaseHoldInput;
import static com.tagokoder.account.infra.in.grpc.validation.InternalAccountsGrpcValidators.toReserveCommand;
import static com.tagokoder.account.infra.in.grpc.validation.InternalAccountsGrpcValidators.toReserveHoldInput;
import static com.tagokoder.account.infra.in.grpc.validation.InternalAccountsGrpcValidators.toValidateCommand;

import com.google.protobuf.StringValue;
import com.tagokoder.account.domain.port.in.BatchGetAccountSummariesUseCase;
import com.tagokoder.account.domain.port.in.OpenAccountWithOpeningBonusUseCase;
import com.tagokoder.account.domain.port.in.ReleaseHoldUseCase;
import com.tagokoder.account.domain.port.in.ReserveHoldUseCase;
import com.tagokoder.account.domain.port.in.ValidateAccountsAndLimitsUseCase;
import com.tagokoder.account.infra.in.grpc.mapper.ProtoEnumMapper;

import bank.accounts.v1.AccountSummary;
import bank.accounts.v1.BatchGetAccountSummariesRequest;
import bank.accounts.v1.BatchGetAccountSummariesResponse;
import bank.accounts.v1.InternalAccountsServiceGrpc;
import bank.accounts.v1.MissingAccount;
import bank.accounts.v1.OpenAccountWithOpeningBonusRequest;
import bank.accounts.v1.OpenAccountWithOpeningBonusResponse;
import bank.accounts.v1.ReleaseHoldRequest;
import bank.accounts.v1.ReleaseHoldResponse;
import bank.accounts.v1.ReserveHoldRequest;
import bank.accounts.v1.ReserveHoldResponse;
import bank.accounts.v1.ValidateAccountsAndLimitsRequest;
import bank.accounts.v1.ValidateAccountsAndLimitsResponse;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;

@GrpcService
public class InternalAccountsGrpcService extends InternalAccountsServiceGrpc.InternalAccountsServiceImplBase {

    private final ValidateAccountsAndLimitsUseCase validateUC;
    private final ReserveHoldUseCase reserveUC;
    private final ReleaseHoldUseCase releaseUC;
    private final BatchGetAccountSummariesUseCase batchSummariesUC;
    private final OpenAccountWithOpeningBonusUseCase openWithBonusUC;

    public InternalAccountsGrpcService(
            ValidateAccountsAndLimitsUseCase validateUC,
            ReserveHoldUseCase reserveUC,
            ReleaseHoldUseCase releaseUC,
            BatchGetAccountSummariesUseCase batchSummariesUC,
            OpenAccountWithOpeningBonusUseCase openWithBonusUC
    ) {
        this.validateUC = validateUC;
        this.reserveUC = reserveUC;
        this.releaseUC = releaseUC;
        this.batchSummariesUC = batchSummariesUC;
        this.openWithBonusUC =openWithBonusUC;
    }

    @Override
    public void openAccountWithOpeningBonus(OpenAccountWithOpeningBonusRequest request,
        StreamObserver<OpenAccountWithOpeningBonusResponse> responseObserver) {

        var cmd = toOpenAccountWithBonusCommand(request);
        var res = openWithBonusUC.open(cmd);

        responseObserver.onNext(OpenAccountWithOpeningBonusResponse.newBuilder()
            .setAccountId(res.accountId().toString())
            .setAccountNumber(res.accountNumber() == null ? "" : res.accountNumber())
            .setBonusJournalId(res.bonusJournalId() == null ? "" : res.bonusJournalId())
            .setStatus(res.status() == null ? "opened" : res.status())
            .build());
        responseObserver.onCompleted();
    }

        @Override
        public void validateAccountsAndLimits(ValidateAccountsAndLimitsRequest request,
                                        StreamObserver<ValidateAccountsAndLimitsResponse> responseObserver) {
        var cmd = toValidateCommand(request);
        var res = validateUC.validate(cmd);

        ValidateAccountsAndLimitsResponse.Builder out = ValidateAccountsAndLimitsResponse.newBuilder()
        .setOk(res.ok());

        if (!res.ok() && res.reasonOrNull() != null) {
        out.setReason(StringValue.of(res.reasonOrNull()));
        }

        responseObserver.onNext(out.build());
        responseObserver.onCompleted();
        }

        @Override
        public void reserveHold(ReserveHoldRequest request, StreamObserver<ReserveHoldResponse> responseObserver) {
        var in = toReserveHoldInput(request);
        var res = reserveUC.reserve(toReserveCommand(in));

        responseObserver.onNext(ReserveHoldResponse.newBuilder()
        .setOk(res.ok())
        .setNewHold(GrpcMoney.str(res.newHold()))
        .build());
        responseObserver.onCompleted();
        }

        @Override
        public void releaseHold(ReleaseHoldRequest request, StreamObserver<ReleaseHoldResponse> responseObserver) {
        var in = toReleaseHoldInput(request);  

        var res = releaseUC.release(toReleaseCommand(in));

        responseObserver.onNext(ReleaseHoldResponse.newBuilder()
        .setOk(res.ok())
        .setNewHold(GrpcMoney.str(res.newHold()))
        .build());
        responseObserver.onCompleted();
        }

    @Override
    public void batchGetAccountSummaries(
            BatchGetAccountSummariesRequest request,
            StreamObserver<BatchGetAccountSummariesResponse> responseObserver) {
        var in = toBatchIdsInput(request);

        var res = batchSummariesUC.batchGetSummaries(new BatchGetAccountSummariesUseCase.Command(in.ids(), in.includeInactive()));
        BatchGetAccountSummariesResponse.Builder out = BatchGetAccountSummariesResponse.newBuilder();

        // accounts OK
        for (var a : res.accounts()) {
            out.addAccounts(AccountSummary.newBuilder()
                    .setAccountId(a.accountId().toString())
                    .setAccountNumber(a.accountNumber())
                    .setProductType(ProtoEnumMapper.mapProductTypeToEnum(a.productType())) // NUEVO mapper
                    .setCurrency(a.currency())
                    .setStatus(a.status())
                    .setDisplayName(a.displayName() == null ? "" : a.displayName())
                    .build());
        }

        for (var m : res.missing()) {
            MissingAccount.Builder mb = MissingAccount.newBuilder()
                    .setAccountId(m.accountId().toString());
            if (m.reason() != null && !m.reason().isBlank()) {
                mb.setReason(StringValue.of(m.reason()));
            }
            out.addMissing(mb.build());
        }

        responseObserver.onNext(out.build());
        responseObserver.onCompleted();
    }
}
