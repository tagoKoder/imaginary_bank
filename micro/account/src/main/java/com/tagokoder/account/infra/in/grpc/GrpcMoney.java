package com.tagokoder.account.infra.in.grpc;

import java.math.BigDecimal;

public final class GrpcMoney {
  private GrpcMoney() {}

  public static BigDecimal bd(Double v) {
    return v == null ? null : BigDecimal.valueOf(v);
  }

  public static BigDecimal bd(double v) {
    return BigDecimal.valueOf(v);
  }

  public static double dbl(BigDecimal v) {
    return v == null ? 0.0d : v.doubleValue();
  }
  public static String str(BigDecimal v) {
    return v == null ? null : v.toPlainString();
  }
}