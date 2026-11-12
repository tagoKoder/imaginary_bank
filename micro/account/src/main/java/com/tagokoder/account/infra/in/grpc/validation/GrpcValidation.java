package com.tagokoder.account.infra.in.grpc.validation;

import com.google.protobuf.DoubleValue;
import com.google.protobuf.StringValue;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.LocalDate;
import java.util.UUID;

public final class GrpcValidation {
    private GrpcValidation() {}

    // -----------------------------
    // Status helpers (sanitizados)
    // -----------------------------
    public static StatusRuntimeException invalid(String msg) {
        return Status.INVALID_ARGUMENT.withDescription(msg).asRuntimeException();
    }
    public static StatusRuntimeException unauth(String msg) {
        return Status.UNAUTHENTICATED.withDescription(msg).asRuntimeException();
    }
    public static StatusRuntimeException forbidden(String msg) {
        return Status.PERMISSION_DENIED.withDescription(msg).asRuntimeException();
    }

    // -----------------------------
    // Strings: trim + límites + anti log-injection
    // -----------------------------
    public static String requireNonBlank(String raw, String field) {
        if (raw == null) throw invalid(field + " is required");
        String s = raw.trim();
        if (s.isEmpty()) throw invalid(field + " is required");
        rejectControlChars(s, field);
        rejectNewlines(s, field);
        return s;
    }

    public static String optionalTrim(String raw, String field, int maxLen) {
        if (raw == null) return null;
        String s = raw.trim();
        if (s.isEmpty()) return null;
        if (s.length() > maxLen) throw invalid(field + " too long");
        rejectControlChars(s, field);
        rejectNewlines(s, field);
        return s;
    }

    public static String optionalTrim(StringValue v, String field, int maxLen) {
        if (v == null) return null;
        return optionalTrim(v.getValue(), field, maxLen);
    }

    public static void rejectControlChars(String s, String field) {
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            if (c < 0x20 || c == 0x7F) {
                throw invalid(field + " contains invalid characters");
            }
        }
    }

    public static void rejectNewlines(String s, String field) {
        if (s.indexOf('\n') >= 0 || s.indexOf('\r') >= 0) {
            throw invalid(field + " contains invalid characters");
        }
    }

    // -----------------------------
    // UUID
    // -----------------------------
    public static UUID requireUuid(String raw, String field) {
        String s = requireNonBlank(raw, field);
        try {
            return UUID.fromString(s);
        } catch (Exception e) {
            throw invalid(field + " must be a valid UUID");
        }
    }

    // -----------------------------
    // ISO-4217 / ISO-3166 (básico)
    // -----------------------------
    public static String requireCurrency(String raw) {
        String s = requireNonBlank(raw, "currency").toUpperCase();
        if (!s.matches("^[A-Z]{3}$")) throw invalid("currency must be ISO-4217 (3 letters)");
        return s;
    }

    public static String requireCountry2(String raw, String field) {
        String s = requireNonBlank(raw, field).toUpperCase();
        if (!s.matches("^[A-Z]{2}$")) throw invalid(field + " must be ISO-3166-1 alpha-2");
        return s;
    }

    // -----------------------------
    // Email / Phone (validación defensiva, no perfecta)
    // -----------------------------
    public static String optionalEmail(String raw, String field) {
        String s = optionalTrim(raw, field, 254);
        if (s == null) return null;
        // simple y defensivo
        if (!s.matches("^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$")) throw invalid(field + " invalid");
        return s;
    }

    public static String optionalPhone(String raw, String field) {
        String s = optionalTrim(raw, field, 32);
        if (s == null) return null;
        if (!s.matches("^[0-9+][0-9()\\-\\s]{5,31}$")) throw invalid(field + " invalid");
        return s;
    }

    // -----------------------------
    // Idempotency / external_ref (anti injection en IDs)
    // -----------------------------
    public static String requireIdempotencyKey(String raw, String field) {
        String s = requireNonBlank(raw, field);
        if (s.length() < 8 || s.length() > 128) throw invalid(field + " length must be 8..128");
        // permitir solo charset seguro para keys (evita payloads raros)
        if (!s.matches("^[A-Za-z0-9._:\\-]+$")) throw invalid(field + " contains invalid characters");
        return s;
    }

    public static String optionalExternalRef(String raw, String field) {
        String s = optionalTrim(raw, field, 128);
        if (s == null) return null;
        if (!s.matches("^[A-Za-z0-9._:\\-/]+$")) throw invalid(field + " contains invalid characters");
        return s;
    }

    // -----------------------------
    // Date
    // -----------------------------
    public static LocalDate requireIsoDate(String raw, String field) {
        String s = requireNonBlank(raw, field);
        try {
            LocalDate d = LocalDate.parse(s);
            if (d.isAfter(LocalDate.now())) throw invalid(field + " cannot be in the future");
            return d;
        } catch (StatusRuntimeException e) {
            throw e;
        } catch (Exception e) {
            throw invalid(field + " must be YYYY-MM-DD");
        }
    }

    // -----------------------------
    // Money / numeric anti-overflow
    // - bloquea NaN/Inf
    // - fuerza max 2 decimales
    // - techo de magnitud (anti abuso/overflow downstream)
    // -----------------------------
    public static BigDecimal requirePositiveMoney(String amount, String field) {
        if (Double.isNaN(Double.parseDouble(amount)) || Double.isInfinite(Double.parseDouble(amount))) throw invalid(field + " invalid");
        if (Double.parseDouble(amount) <= 0d) throw invalid(field + " must be > 0");

        BigDecimal bd = BigDecimal.valueOf(Double.parseDouble(amount));
        // max 2 decimales sin redondear (evita “1.99999997”)
        try {
            bd = bd.setScale(2, RoundingMode.UNNECESSARY);
        } catch (ArithmeticException ex) {
            throw invalid(field + " must have max 2 decimals");
        }

        // techo anti abuso (ajústalo)
        if (bd.compareTo(new BigDecimal("1000000000.00")) > 0) {
            throw invalid(field + " too large");
        }
        return bd;
    }

    public static BigDecimal optionalNonNegativeMoney(String v, String field) {
        if (v == null) return null;
        double raw = Double.parseDouble(v);
        if (Double.isNaN(raw) || Double.isInfinite(raw)) throw invalid(field + " invalid");
        if (raw < 0d) throw invalid(field + " must be >= 0");

        BigDecimal bd = BigDecimal.valueOf(raw);
        try {
            bd = bd.setScale(2, RoundingMode.UNNECESSARY);
        } catch (ArithmeticException ex) {
            throw invalid(field + " must have max 2 decimals");
        }

        if (bd.compareTo(new BigDecimal("1000000000.00")) > 0) {
            throw invalid(field + " too large");
        }
        return bd;
    }

    // -----------------------------
    // Account number (1..12 dígitos)
    // -----------------------------
    public static String requireAccountNumber12(String raw, String field) {
        String s = requireNonBlank(raw, field);
        if (!s.matches("^\\d{1,12}$")) throw invalid(field + " must be numeric up to 12 digits");
        return s;
    }
}