package com.tagokoder.account.infra.security.oauth;

import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Base64;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

@Component
public class ServiceTokenProvider {

    private final HttpClient http = HttpClient.newHttpClient();
    private final ObjectMapper mapper = new ObjectMapper();

    @Value("${security.service-token.url}")
    private String tokenUrl;

    @Value("${security.service-token.client-id}")
    private String clientId;

    @Value("${security.service-token.client-secret}")
    private String clientSecret;

    @Value("${security.service-token.scope}")
    private String scope;

    private String cachedToken;
    private Instant expiresAt = Instant.EPOCH;

    public synchronized String getAccessToken() {
        if (cachedToken != null &&
            Instant.now().isBefore(expiresAt.minusSeconds(60))) {
            return cachedToken;
        }

        if (blank(tokenUrl) || blank(clientId) ||
            blank(clientSecret) || blank(scope)) {
            throw new IllegalStateException(
                "Service OAuth configuration is incomplete"
            );
        }

        try {
            String basic = Base64.getEncoder().encodeToString(
                (clientId + ":" + clientSecret)
                    .getBytes(StandardCharsets.UTF_8)
            );

            String body =
                "grant_type=client_credentials" +
                "&scope=" + encode(scope);

            HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(tokenUrl))
                .header("Authorization", "Basic " + basic)
                .header("Content-Type", "application/x-www-form-urlencoded")
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .build();

            HttpResponse<String> response = http.send(
                request,
                HttpResponse.BodyHandlers.ofString()
            );

            if (response.statusCode() < 200 ||
                response.statusCode() >= 300) {
                throw new IllegalStateException(
                    "OAuth token endpoint returned HTTP " +
                    response.statusCode()
                );
            }

            JsonNode json = mapper.readTree(response.body());
            String token = json.path("access_token").asText("");
            long expiresIn = json.path("expires_in").asLong(0);

            if (blank(token) || expiresIn <= 0) {
                throw new IllegalStateException(
                    "OAuth token response is incomplete"
                );
            }

            cachedToken = token;
            expiresAt = Instant.now().plusSeconds(expiresIn);
            return cachedToken;

        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException(
                "Interrupted while requesting service token", e
            );
        } catch (Exception e) {
            throw new IllegalStateException(
                "Unable to obtain service access token", e
            );
        }
    }

    private static String encode(String value) {
        return URLEncoder.encode(value, StandardCharsets.UTF_8);
    }

    private static boolean blank(String value) {
        return value == null || value.isBlank();
    }
}