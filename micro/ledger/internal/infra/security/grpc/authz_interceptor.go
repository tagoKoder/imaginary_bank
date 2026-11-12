package securitygrpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	vptypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
	"github.com/google/uuid"
	"github.com/tagoKoder/ledger/internal/domain/port/out"
	"github.com/tagoKoder/ledger/internal/infra/security/authz"
	"github.com/tagoKoder/ledger/internal/infra/security/avp"
	authctx "github.com/tagoKoder/ledger/internal/infra/security/context"
	jwtvalidator "github.com/tagoKoder/ledger/internal/infra/security/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Interceptor struct {
	jwt      *jwtvalidator.Validator
	avp      *avp.Client
	registry *authz.Registry
	tpl      *authz.Templates
	audit    out.AuditPort

	hashSaltIP string
	hashSaltUA string
}

func NewAuthzInterceptor(
	jwtv *jwtvalidator.Validator,
	avpc *avp.Client,
	reg *authz.Registry,
	tpl *authz.Templates,
	audit out.AuditPort,
	hashSaltIP, hashSaltUA string,
) *Interceptor {
	return &Interceptor{
		jwt: jwtv, avp: avpc, registry: reg, tpl: tpl, audit: audit,
		hashSaltIP: hashSaltIP, hashSaltUA: hashSaltUA,
	}
}

func (i *Interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {

		// correlation id
		cid := readIncoming(ctx, "x-correlation-id")
		if cid == "" {
			cid = uuid.NewString()
		}
		ctx = authctx.WithCorrelationID(ctx, cid)
		_ = grpc.SetTrailer(ctx, metadata.Pairs("x-correlation-id", cid))

		// bypass infra
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") ||
			strings.HasPrefix(info.FullMethod, "/grpc.reflection.v1alpha.ServerReflection/") {
			return handler(ctx, req)
		}

		def, ok := i.registry.Get(info.FullMethod)
		if !ok {
			i.bestEffortAudit(ctx, info.FullMethod, "DENY", "route_not_registered", time.Now().UTC(), nil)
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}

		// PUBLIC: solo para operaciones explícitamente públicas.
		if def.Mode == authz.ModePublic {
			return handler(ctx, req)
		}

		authzHdr := readIncoming(ctx, "authorization")

		// SERVICE_ONLY: token de cliente OAuth autorizado y scope exacto.
		if def.Mode == authz.ModeServiceOnly {
			claims, err := i.jwt.ValidateServiceBearer(ctx, authzHdr)
			if err != nil {
				i.bestEffortAudit(
					ctx,
					info.FullMethod,
					"DENY",
					"service_authn_failed",
					time.Now().UTC(),
					nil,
				)
				return nil, status.Error(codes.PermissionDenied, "forbidden")
			}

			ctx = authctx.WithActionID(ctx, def.ActionID)
			ctx = authctx.WithRouteTemplate(ctx, "CALL "+info.FullMethod)

			resp, handlerErr := handler(ctx, req)

			outcome := "ok"
			if handlerErr != nil {
				outcome = "handler_error"
			}
			i.bestEffortAudit(
				ctx,
				info.FullMethod,
				"ALLOW",
				outcome,
				time.Now().UTC(),
				map[string]any{
					"client_id": claims.ClientID,
					"action":    def.ActionID,
				},
			)

			return resp, handlerErr
		}

		// Token de usuario requerido para AUTHN_ONLY/AUTHZ.
		claims, err := i.jwt.ValidateBearer(ctx, authzHdr)

		if err != nil {
			i.bestEffortAudit(ctx, info.FullMethod, "DENY", "authn_failed", time.Now().UTC(), map[string]any{"err": err.Error()})
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		ctx = authctx.WithAccessToken(ctx, bearerValue(authzHdr))
		ctx = authctx.WithActor(ctx, authctx.Actor{
			Subject:    claims.Subject,
			CustomerID: claims.CustomerID,
			Roles:      claims.Roles,
			MFA:        claims.MFA,
		})

		// context (hash)
		ip := readIncoming(ctx, "x-forwarded-for")
		ua := readIncoming(ctx, "user-agent")
		ctx = authctx.WithIPHash(ctx, shaHex(ip+i.hashSaltIP))
		ctx = authctx.WithUAHash(ctx, shaHex(ua+i.hashSaltUA))

		// route template
		ctx = authctx.WithRouteTemplate(ctx, "CALL "+info.FullMethod)
		ctx = authctx.WithActionID(ctx, def.ActionID)

		// idempotency key: intenta leer campo genérico
		if k := authz.ExtractStringField(req, "IdempotencyKey"); k != "" {
			ctx = authctx.WithIdempotencyKey(ctx, k)
		} else if k := authz.ExtractStringField(req, "idempotency_key"); k != "" {
			ctx = authctx.WithIdempotencyKey(ctx, k)
		}

		// AUTHN_ONLY
		if def.Mode == authz.ModeAuthnOnly {
			return handler(ctx, req)
		}

		// AUTHZ
		resolved, err := i.tpl.Resolve(ctx, def.ResourceTemplate, req)
		if err != nil {
			i.bestEffortAudit(ctx, info.FullMethod, "DENY", "resource_resolve_error", time.Now().UTC(), map[string]any{"err": err.Error()})
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}

		actor := authctx.ActorFrom(ctx)
		if def.RequireCustomerLink && actor.CustomerID == "" {
			i.bestEffortAudit(ctx, info.FullMethod, "DENY", "customer_link_missing", time.Now().UTC(), nil)
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}

		avpCtx := map[string]vptypes.AttributeValue{
			"correlation_id": &vptypes.AttributeValueMemberString{Value: cid},
			"route_template": &vptypes.AttributeValueMemberString{Value: authctx.RouteTemplate(ctx)},
			"ip_hash":        &vptypes.AttributeValueMemberString{Value: authctx.IPHash(ctx)},
			"ua_hash":        &vptypes.AttributeValueMemberString{Value: authctx.UAHash(ctx)},
		}
		if actor.CustomerID != "" {
			avpCtx["customer_id"] = &vptypes.AttributeValueMemberString{Value: actor.CustomerID}
		}
		if resolved.OwnerCustomerID != "" {
			avpCtx["owner_customer_id"] = &vptypes.AttributeValueMemberString{Value: resolved.OwnerCustomerID}
		}
		if ik := authctx.IdempotencyKey(ctx); ik != "" {
			avpCtx["idempotency_key"] = &vptypes.AttributeValueMemberString{Value: ik}
		}
		for k, v := range resolved.ContextAttrs {
			avpCtx[k] = v
		}

		dec, err := i.avp.AuthorizeWithToken(ctx, authctx.AccessToken(ctx), authz.Action{ID: def.ActionID, RouteTemplate: authctx.RouteTemplate(ctx)}, authz.Resource{
			Type: resolved.ResourceType, ID: resolved.ResourceID, OwnerCustomerID: resolved.OwnerCustomerID,
		}, avpCtx)
		if err != nil {
			i.bestEffortAudit(ctx, info.FullMethod, "DENY", "avp_error", time.Now().UTC(), map[string]any{"err": err.Error()})
			return nil, status.Error(codes.Internal, "authorization error")
		}

		ctx = authctx.WithAuthzDecision(ctx, string(dec.Decision))
		if dec.PolicyID != "" {
			ctx = authctx.WithAuthzPolicyID(ctx, dec.PolicyID)
		}
		if dec.Decision != avp.DecisionAllow {
			i.bestEffortAudit(ctx, info.FullMethod, "DENY", "not_authorized", time.Now().UTC(), map[string]any{
				"action": def.ActionID, "resource": map[string]any{"type": resolved.ResourceType, "id": resolved.ResourceID},
			})
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}

		resp, hErr := handler(ctx, req)

		if def.Critical {
			outcome := "ok"
			if hErr != nil {
				outcome = "handler_error"
			}
			i.bestEffortAudit(ctx, info.FullMethod, "ALLOW", outcome, time.Now().UTC(), map[string]any{"action": def.ActionID})
		}

		return resp, hErr
	}
}

func (i *Interceptor) bestEffortAudit(ctx context.Context, fullMethod, decision, outcome string, at time.Time, extra map[string]any) {
	if i.audit == nil {
		return
	}
	_ = i.audit.Record(ctx, "AUTHZ", fullMethod, at, map[string]any{
		"decision":       decision,
		"outcome":        outcome,
		"correlation_id": authctx.CorrelationID(ctx),
		"route_template": authctx.RouteTemplate(ctx),
		"action_id":      authctx.ActionID(ctx),
		"policy_id":      authctx.AuthzPolicyID(ctx),
		"ip_hash":        authctx.IPHash(ctx),
		"ua_hash":        authctx.UAHash(ctx),
		"extra":          extra,
	})
}

func readIncoming(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vs := md.Get(strings.ToLower(key))
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}
func bearerValue(header string) string {
	h := strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[len("bearer "):])
	}
	return ""
}
func shaHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
