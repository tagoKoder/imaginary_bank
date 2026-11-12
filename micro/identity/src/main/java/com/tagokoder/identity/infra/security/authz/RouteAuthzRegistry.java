package com.tagokoder.identity.infra.security.authz;

import java.util.Map;

public class RouteAuthzRegistry {

  // Templates
  public static final String T_SYSTEM          = "SYSTEM";
  public static final String T_REGISTRATION_ID = "REGISTRATION_ID";
  public static final String T_SESSION_ID      = "SESSION_ID";

  private final Map<String, RouteDef> routes = Map.ofEntries(
      // ---- Infra ---- (health/reflection se bypass en interceptor)

      // ---- Onboarding (PUBLIC) ----
      Map.entry("bank.identity.v1.OnboardingService/StartRegistration",
          new RouteDef("identity.onboarding.start_registration", true, AuthzMode.PUBLIC, T_REGISTRATION_ID)),
      Map.entry("bank.identity.v1.OnboardingService/ConfirmRegistrationKyc",
          new RouteDef("identity.onboarding.confirm_kyc", true, AuthzMode.PUBLIC, T_REGISTRATION_ID)),
      Map.entry("bank.identity.v1.OnboardingService/ActivateRegistration",
          new RouteDef("identity.onboarding.activate_registration", true, AuthzMode.AUTHN_ONLY, T_REGISTRATION_ID)),

      // ---- OIDC (PUBLIC) ----
      Map.entry("bank.identity.v1.OidcAuthService/StartOidcLogin",
          new RouteDef("identity.oidc.start_login", false, AuthzMode.PUBLIC, T_SYSTEM)),
      Map.entry("bank.identity.v1.OidcAuthService/CompleteOidcLogin",
          new RouteDef("identity.oidc.complete_login", true, AuthzMode.PUBLIC, T_SYSTEM)),

      // ---- Session (AUTHN_ONLY) ----
      Map.entry("bank.identity.v1.OidcAuthService/RefreshSession",
          new RouteDef("identity.session.refresh", true, AuthzMode.PUBLIC, T_SESSION_ID)),
      Map.entry("bank.identity.v1.OidcAuthService/LogoutSession",
          new RouteDef("identity.session.logout", true, AuthzMode.PUBLIC, T_SESSION_ID)),
      Map.entry("bank.identity.v1.OidcAuthService/GetSessionInfo",
          new RouteDef("identity.session.get_info", false, AuthzMode.PUBLIC, T_SESSION_ID)),

      // ---- Principal (AUTHN_ONLY) ----
      // esto fuerza a que Account→Identity siempre mande Bearer
      Map.entry("bank.identity.v1.PrincipalService/ResolvePrincipal",
          new RouteDef("identity.principal.resolve", false, AuthzMode.AUTHN_ONLY, T_SYSTEM))
  );

  public RouteDef get(String fullMethodName) {
    return routes.get(fullMethodName); // null => fail-closed
  }
}