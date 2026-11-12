package com.tagokoder.identity.infra.in.grpc.validation;

import io.grpc.Status;
import io.grpc.StatusRuntimeException;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.LocalDate;

public final class GrpcValidation {
  private GrpcValidation() {}

  // -----------------------------
  // Exceptions helpers
  // -----------------------------
  public static StatusRuntimeException invalid(String msg) {
    return Status.INVALID_ARGUMENT.withDescription(msg).asRuntimeException();
  }

  public static StatusRuntimeException unauthenticated(String msg) {
    return Status.UNAUTHENTICATED.withDescription(msg).asRuntimeException();
  }

  public static StatusRuntimeException forbidden(String msg) {
    return Status.PERMISSION_DENIED.withDescription(msg).asRuntimeException();
  }

  // -----------------------------
  // Basic string guards
  // -----------------------------
  public static String requireNonBlank(String s, String field) {
    if (s == null) throw invalid(field + " is required");
    String t = s.trim();
    if (t.isEmpty()) throw invalid(field + " is required");
    // bloquea control chars (inyección en logs/headers)
    if (containsControlChars(t)) throw invalid(field + " contains invalid characters");
    return t;
  }

  public static String optionalTrim(String s, String field, int maxLen) {
    if (s == null) return null;
    String t = s.trim();
    if (t.isEmpty()) return null;
    if (containsControlChars(t)) throw invalid(field + " contains invalid characters");
    if (t.length() > maxLen) throw invalid(field + " too long");
    return t;
  }

  private static boolean containsControlChars(String s) {
    for (int i = 0; i < s.length(); i++) {
      char c = s.charAt(i);
      if (c <= 0x1F || c == 0x7F) return true;
    }
    return false;
  }

  // -----------------------------
  // UUID / dates
  // -----------------------------
  public static java.util.UUID requireUuid(String raw, String field) {
    String s = requireNonBlank(raw, field);
    if (s.length() > 64) throw invalid(field + " too long");
    try {
      return java.util.UUID.fromString(s);
    } catch (Exception e) {
      throw invalid(field + " must be a UUID");
    }
  }

  public static LocalDate requireIsoDate(String raw, String field) {
    String s = requireNonBlank(raw, field);
    if (s.length() != 10) throw invalid(field + " must be YYYY-MM-DD");
    // valida forma básica antes de parse
    if (!s.matches("^\\d{4}-\\d{2}-\\d{2}$")) throw invalid(field + " must be YYYY-MM-DD");
    try {
      return LocalDate.parse(s);
    } catch (Exception e) {
      throw invalid(field + " invalid date");
    }
  }

  // -----------------------------
  // Email / phone
  // -----------------------------
  public static String optionalEmail(String raw, String field) {
    String s = optionalTrim(raw, field, 254);
    if (s == null) return null;
    // simple, defensivo (evita payloads raros / inyección)
    if (!s.matches("^[A-Za-z0-9._%+\\-]+@[A-Za-z0-9.\\-]+\\.[A-Za-z]{2,}$")) throw invalid(field + " invalid");
    return s;
  }

  public static String optionalPhone(String raw, String field) {
    String s = optionalTrim(raw, field, 32);
    if (s == null) return null;
    // E.164-like (defensivo)
    if (!s.matches("^\\+?[0-9]{7,20}$")) throw invalid(field + " invalid");
    return s;
  }

  public static String requireCountry2(String raw, String field) {
    String s = requireNonBlank(raw, field);
    if (!s.matches("^[A-Za-z]{2}$")) throw invalid(field + " must be ISO-3166-1 alpha-2");
    return s.toUpperCase();
  }

  // -----------------------------
  // Channel
  // -----------------------------
  public static String requireChannel(String raw) {
    String s = requireNonBlank(raw, "channel").toLowerCase();
    if (s.length() > 16) throw invalid("channel too long");
    if (!s.matches("^[a-z0-9_-]{2,16}$")) throw invalid("channel invalid");
    return s;
  }

  // -----------------------------
  // Redirect-after-login (Open Redirect defense)
  // -----------------------------
  public static String requireSafeRedirectPath(String raw) {
    String s = requireNonBlank(raw, "redirect_after_login");
    if (s.length() > 256) throw invalid("redirect_after_login too long");
    // solo rutas relativas internas
    if (!s.startsWith("/")) throw invalid("redirect_after_login must start with /");
    // bloquea esquema/host/CRLF
    String low = s.toLowerCase();
    if (low.startsWith("//") || low.contains("://")) throw invalid("redirect_after_login invalid");
    if (s.contains("\\") || s.contains("\r") || s.contains("\n")) throw invalid("redirect_after_login invalid");
    return s;
  }

  // -----------------------------
  // OIDC code/state (defensivo)
  // -----------------------------
  public static String requireOidcCode(String raw) {
    String s = requireNonBlank(raw, "code");
    if (s.length() > 2048) throw invalid("code too long");
    // visible ASCII (sin espacios/control)
    if (!s.matches("^[\\x21-\\x7E]+$")) throw invalid("code invalid");
    return s;
  }

  public static String requireOidcState(String raw) {
    String s = requireNonBlank(raw, "state");
    if (s.length() > 512) throw invalid("state too long");
    if (!s.matches("^[\\x21-\\x7E]+$")) throw invalid("state invalid");
    return s;
  }

  // -----------------------------
  // IP / UA (no PII cruda, pero si viene, guard defensivo)
  // -----------------------------
  public static String optionalClientHint(String raw, String field, int maxLen) {
    String s = optionalTrim(raw, field, maxLen);
    if (s == null) return "";
    // evita cosas raras en logs/headers
    if (!s.matches("^[\\x20-\\x7E]+$")) throw invalid(field + " invalid");
    return s;
  }

  // -----------------------------
  // Money / doubles (NaN/Inf/overflow)
  // -----------------------------
  public static BigDecimal requireNonNegativeMoney(String v, String field) {
    if (Double.isNaN(Double.parseDouble(v)) || Double.isInfinite(Double.parseDouble(v))) throw invalid(field + " invalid number");
    if (Double.parseDouble(v) < 0d) throw invalid(field + " must be >= 0");
    // límite DoS/overflow (ajústalo si quieres)
    if (Double.parseDouble(v) > 1_000_000_000d) throw invalid(field + " too large");
    // normaliza a 2 decimales para evitar problemas de precisión
    return new BigDecimal(v).setScale(2, RoundingMode.HALF_UP);
  }

  // -----------------------------
  // MIME type hints
  // -----------------------------
  public static String optionalMimeType(String raw, String field) {
    String s = optionalTrim(raw, field, 100);
    if (s == null) return "";
    if (!s.matches("^[a-zA-Z0-9!#$&^_.+-]+/[a-zA-Z0-9!#$&^_.+-]+$")) throw invalid(field + " invalid");
    return s.toLowerCase();
  }

  // -----------------------------
  // National ID / fingerprint (defensivo)
  // -----------------------------
  public static String requireNationalId(String raw) {
    String s = requireNonBlank(raw, "national_id");
    if (s.length() > 32) throw invalid("national_id too long");
    if (!s.matches("^[A-Za-z0-9\\-]+$")) throw invalid("national_id invalid");
    return s;
  }

  public static String requireFingerprintCode(String raw) {
    String s = requireNonBlank(raw, "fingerprint_code");
    if (s.length() > 64) throw invalid("fingerprint_code too long");
    if (!s.matches("^[A-Za-z0-9\\-]+$")) throw invalid("fingerprint_code invalid");
    return s;
  }

  // -----------------------------
  // S3 bucket/key/etag (Confirm KYC)
  // -----------------------------
  public static String requireS3Bucket(String raw, String field) {
    String s = requireNonBlank(raw, field);
    if (s.length() > 63) throw invalid(field + " too long");
    // bucket safe-ish (no perfecto, pero defensivo)
    if (!s.matches("^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$")) throw invalid(field + " invalid");
    if (s.contains("..")) throw invalid(field + " invalid");
    return s;
  }

  public static String requireS3Key(String raw, String field) {
    String s = requireNonBlank(raw, field);
    if (s.length() > 1024) throw invalid(field + " too long");
    // bloquea control chars
    if (s.contains("\r") || s.contains("\n")) throw invalid(field + " invalid");
    return s;
  }

  public static String optionalEtag(String raw, String field) {
    String s = optionalTrim(raw, field, 80);
    if (s == null) return "";
    // ETag común: "hex" o hex-multipart
    String unq = s.replace("\"", "");
    if (!unq.matches("^[A-Fa-f0-9]{32}(?:-\\d+)?$")) throw invalid(field + " invalid");
    return s;
  }

  public static void requireAcceptedTerms(boolean accepted) {
    if (!accepted) throw invalid("accepted_terms must be true");
  }
}