package com.tagokoder.identity.application.service;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.tagokoder.identity.application.IdentityClientsProperties;
import com.tagokoder.identity.domain.model.RegistrationIntent;
import com.tagokoder.identity.domain.model.kyc.FinalizedObject;
import com.tagokoder.identity.domain.port.in.ActivateRegistrationUseCase;
import com.tagokoder.identity.domain.port.in.ConfirmRegistrationKycUseCase;
import com.tagokoder.identity.domain.port.in.StartRegistrationUseCase;
import com.tagokoder.identity.domain.port.out.AccountsClientPort;
import com.tagokoder.identity.domain.port.out.IdentityLinkRepositoryPort;
import com.tagokoder.identity.domain.port.out.KycPresignedStoragePort;
import com.tagokoder.identity.domain.port.out.RegistrationIntentRepositoryPort;
import com.tagokoder.identity.domain.port.out.IdentityRepositoryPort;
import com.tagokoder.identity.infra.security.context.AuthCtx;

@Service
public class OnboardingService implements StartRegistrationUseCase, ConfirmRegistrationKycUseCase, ActivateRegistrationUseCase {

  private final RegistrationIntentRepositoryPort registrationRepo;
  private final KycPresignedStoragePort kycStorage;

  private final AccountsClientPort accounts;
  private final IdentityClientsProperties clientProps;
  private final IdentityLinkRepositoryPort identityLinks;
  private final IdentityRepositoryPort identityRepository;

  public OnboardingService(
    RegistrationIntentRepositoryPort registrationRepo,
    KycPresignedStoragePort kycStorage,
    AccountsClientPort accounts,
    IdentityClientsProperties clientProps,
    IdentityLinkRepositoryPort identityLinks,
    IdentityRepositoryPort identityRepository
  ) {
    this.registrationRepo = registrationRepo;
    this.kycStorage = kycStorage;
    this.accounts = accounts;
    this.clientProps = clientProps;
    this.identityLinks = identityLinks;
    this.identityRepository = identityRepository;
  }

  @Override
  @Transactional
  public StartRegistrationResponse start(StartRegistrationCommand c) {
    UUID regId = UUID.randomUUID();
    Instant now = Instant.now();

    var uploads = kycStorage.issuePresignedUploads(regId, c.idFrontContentType(), c.selfieContentType());

    RegistrationIntent reg = new RegistrationIntent();
    reg.setId(regId);
    reg.setState(RegistrationIntent.State.STARTED);
    reg.setCreatedAt(now);
    reg.setUpdatedAt(now);

    reg.setChannel(c.channel());
    reg.setEmail(c.email());
    reg.setPhone(c.phone());

    reg.setNationalId(c.nationalId());
    reg.setNationalIdIssueDate(c.nationalIdIssueDate());
    reg.setFingerprintCode(c.fingerprintCode());

    reg.setMonthlyIncome(c.monthlyIncome());
    reg.setOccupationType(c.occupationType());

    registrationRepo.save(reg);

    return new StartRegistrationResponse(regId, "started", now, uploads);
  }

  @Override
  @Transactional
  public ConfirmRegistrationKycResult confirm(ConfirmRegistrationKycCommand command) {
    UUID regId = command.registrationId();
    if (regId == null) throw new IllegalArgumentException("registrationId required");

    RegistrationIntent reg = registrationRepo.findById(regId)
      .orElseThrow(() -> new IllegalStateException("registration not found"));

    List<FinalizedObject> finalized = kycStorage.confirmAndFinalize(regId, command.objects());

    for (FinalizedObject f : finalized) {
      reg.getKycObjects().removeIf(x -> x.kind() == f.kind());
      reg.addKycObject(f);
    }

    reg.setState(RegistrationIntent.State.KYC_CONFIRMED);
    reg.setUpdatedAt(Instant.now());
    registrationRepo.save(reg);

    return new ConfirmRegistrationKycResult(regId, "kyc_confirmed", reg.getUpdatedAt(), finalized);
  }

  @Override
  public ActivateRegistrationResult activate(ActivateRegistrationCommand c) {
    UUID regId = c.registrationId();

    RegistrationIntent reg = registrationRepo.findById(regId)
      .orElseThrow(() -> new IllegalStateException("registration not found"));

    assertRegistrationOwner(reg);

    var principal = AuthCtx.PRINCIPAL.get();

    if (principal == null
            || principal.sub() == null
            || principal.sub().isBlank()) {
        throw new SecurityException("Authenticated principal required");
    }

    var identity = identityRepository
        .findBySubjectAndProvider(principal.sub(), "cognito")
        .orElseThrow(() ->
            new SecurityException("Identity not found"));

    if (reg.getIdentityId() == null
            || !reg.getIdentityId().equals(identity.getId())) {
        throw new SecurityException(
            "Registration does not belong to authenticated identity");
    }

    if (reg.getState() == RegistrationIntent.State.ACTIVATED) {
      return buildResponse(reg, c.correlationId());
    }
    if (reg.getState() != RegistrationIntent.State.KYC_CONFIRMED &&
        reg.getState() != RegistrationIntent.State.ACTIVATING) {
      throw new IllegalStateException("registration must be KYC_CONFIRMED to activate");
    }
    if (!c.acceptedTerms()) throw new IllegalArgumentException("accepted_terms required");

    if (reg.getActivationRef() == null || reg.getActivationRef().isBlank()) {
      reg.setActivationRef("act:" + regId);
    }
    reg.setState(RegistrationIntent.State.ACTIVATING);
    reg.setUpdatedAt(Instant.now());
    registrationRepo.save(reg);

    final String bearer = clientProps.getServiceAccessToken(); // token de servicio
    final String activationRef = reg.getActivationRef();

    // (1) CreateCustomer idempotente (igual que antes)
    if (reg.getCustomerId() == null || reg.getCustomerId().isBlank()) {
      String customerId = accounts.createCustomer(
        bearer,
        activationRef + ":customer",
        regId.toString(),
        c.fullName(),
        c.birthDate(),
        c.tin(),
        c.email(),
        c.phone()
      );
      reg.setCustomerId(customerId);
      reg.setUpdatedAt(Instant.now());
      registrationRepo.save(reg);

      identityLinks.upsert(reg.getIdentityId(), reg.getCustomerId());
    }

    // (2) OPEN SAVING + BONUS (UNA sola llamada idempotente)
    // Esto reemplaza tu antiguo CreateAccount + ledger.creditAccount
    boolean needsOpen = isBlank(reg.getPrimaryAccountId()) || isBlank(reg.getBonusJournalId());
    if (needsOpen) {
      String idem = activationRef + ":savings"; // estable
      String ext  = activationRef + ":savings"; // estable (pasa validadores: empieza con act:)

      AccountsClientPort.OpenedAccount opened = accounts.openAccountWithOpeningBonus(
        bearer,
        reg.getCustomerId(),
        "USD",
        "SAVINGS",
        idem,
        ext,
        "svc:identity"
      );

      if (!isBlank(opened.accountId())) reg.setPrimaryAccountId(opened.accountId());
      if (!isBlank(opened.bonusJournalId())) reg.setBonusJournalId(opened.bonusJournalId());
      reg.setUpdatedAt(Instant.now());
      registrationRepo.save(reg);
    }

    if (isBlank(reg.getCustomerId())
            || isBlank(reg.getPrimaryAccountId())
            || isBlank(reg.getBonusJournalId())) {
        throw new IllegalStateException(
            "Cannot activate registration: account or bonus is incomplete"
        );
    }

    reg.setState(RegistrationIntent.State.ACTIVATED);
    reg.setActivatedAt(Instant.now());
    reg.setUpdatedAt(Instant.now());
    registrationRepo.save(reg);

    return buildResponse(reg, c.correlationId());
  }

  private ActivateRegistrationResult buildResponse(RegistrationIntent reg, String correlationId) {
    List<ActivatedAccount> accounts = new ArrayList<>();
    if (!isBlank(reg.getPrimaryAccountId())) {
      accounts.add(new ActivatedAccount(reg.getPrimaryAccountId(), "USD", "CHECKING"));
    }
    if (!isBlank(reg.getSavingsAccountId())) {
      accounts.add(new ActivatedAccount(reg.getSavingsAccountId(), "USD", "SAVINGS"));
    }

    return new ActivateRegistrationResult(
      reg.getId(),
      reg.getState().name(),
      reg.getCustomerId(),
      reg.getPrimaryAccountId(),
      reg.getActivationRef(),
      accounts,
      reg.getBonusJournalId(),
      correlationId == null ? "" : correlationId
    );
  }

  private static boolean isBlank(String s) {
    return s == null || s.isBlank();
  }
  private void assertRegistrationOwner(RegistrationIntent reg) {
    AuthCtx.AuthPrincipal principal = AuthCtx.PRINCIPAL.get();

    if (principal == null ||
        principal.sub() == null ||
        principal.sub().isBlank()) {
        throw new SecurityException("Authentication required");
    }

    if (reg.getIdentityId() == null) {
        throw new SecurityException(
            "Registration is not linked to an authenticated identity"
        );
    }

    var identity = identityRepository
        .findBySubjectAndProvider(principal.sub(), "cognito")
        .orElseThrow(() ->
            new SecurityException("Authenticated identity not found")
        );

    if (!identity.getId().equals(reg.getIdentityId())) {
        throw new SecurityException(
            "Authenticated user does not own this registration"
        );
    }
}
}