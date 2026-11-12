package com.tagokoder.identity.infra.security.authz;

import java.util.Map;

public class ActionResolver {

    public record ActionDef(String actionId, boolean critical, boolean publicUnauthenticated) {}

    private final Map<String, ActionDef> map = Map.ofEntries(
            // Onboarding (public, sin Bearer)
            Map.entry("bank.identity.v1.OnboardingService/StartRegistration",
                    new ActionDef("identity.onboarding.start_registration", true, true)),
            Map.entry("bank.identity.v1.OnboardingService/ConfirmRegistrationKyc",
                    new ActionDef("identity.onboarding.confirm_kyc", true, true)),
            Map.entry("bank.identity.v1.OnboardingService/ActivateRegistration",
                    new ActionDef("identity.onboarding.activate_registration", true, false)),

            // OIDC (public, sin Bearer)
            Map.entry("bank.identity.v1.OidcAuthService/StartOidcLogin",
                    new ActionDef("identity.oidc.start_login", false, true)),
            Map.entry("bank.identity.v1.OidcAuthService/CompleteOidcLogin",
                    new ActionDef("identity.oidc.complete_login", true, true)),

            // Session (aquí sí puedes exigir Bearer si lo decides; si tu BFF llama con token, perfecto)
            Map.entry("bank.identity.v1.OidcAuthService/RefreshSession",
                    new ActionDef("identity.session.refresh", true, true)),
            Map.entry("bank.identity.v1.OidcAuthService/LogoutSession",
                    new ActionDef("identity.session.logout", true, true)),
            Map.entry("bank.identity.v1.OidcAuthService/GetSessionInfo",
                    new ActionDef("identity.session.get_info", false, true)),

            // Principal Service
            Map.entry("bank.identity.v1.PrincipalService/ResolvePrincipal",
                    new ActionDef("identity.principal.resolve", false, true))
    );

    public ActionDef resolve(String fullMethodName) {
        return map.getOrDefault(fullMethodName, new ActionDef("unknown", true, false));
    }
}
