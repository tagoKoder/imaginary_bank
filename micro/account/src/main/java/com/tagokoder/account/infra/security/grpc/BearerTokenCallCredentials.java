package com.tagokoder.account.infra.security.grpc;

import java.util.concurrent.Executor;

import io.grpc.CallCredentials;
import io.grpc.Metadata;
import io.grpc.Status;

public final class BearerTokenCallCredentials extends CallCredentials {

    private static final Metadata.Key<String> AUTHORIZATION =
        Metadata.Key.of(
            "authorization",
            Metadata.ASCII_STRING_MARSHALLER
        );

    private final String token;

    public BearerTokenCallCredentials(String token) {
        if (token == null || token.isBlank()) {
            throw new IllegalArgumentException(
                "Service access token is required"
            );
        }
        this.token = token;
    }

    @Override
    public void applyRequestMetadata(
            RequestInfo requestInfo,
            Executor appExecutor,
            MetadataApplier applier) {

        appExecutor.execute(() -> {
            Metadata headers = new Metadata();
            headers.put(
                AUTHORIZATION,
                "Bearer " + token
            );
            applier.apply(headers);
        });
    }

    @Override
    public void thisUsesUnstableApi() {
        // Required by the current gRPC CallCredentials API.
    }
}
