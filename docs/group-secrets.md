# Group secrets

FleetAMP group secrets hold passwords, tokens and other sensitive configuration values for one ownership group. Only an administrator or an assigned Group Owner can add, replace or delete a key. Values are write-only in the console and encrypted at rest; the page lists only key names and update metadata.

## Use a secret in Collector configuration

Open **Groups & Labels**, choose a group, then select **Secrets**. Add a key such as `otlp_token`. Reference it in configuration with a quoted placeholder:

```yaml
exporters:
  otlphttp/production:
    endpoint: https://otel.example.com
    headers:
      Authorization: "Bearer ${secret:otlp_token}"
```

Using quotes is recommended so values containing YAML punctuation remain scalar text after substitution.

FleetAMP validates that every referenced key exists without decrypting its value. It resolves the placeholder only when constructing the OpAMP delivery payload. The immutable configuration version keeps the placeholder. The resolved payload receives its own content hash, which is used for delivery status and drift comparison.

If a referenced key is missing, validation or deployment is blocked with the key name; FleetAMP does not send a partial configuration.

## Exposure boundaries

Secret values are not included in:

- configuration versions or Blueprint previews;
- approval diffs or audit-event details;
- group drift views or user-facing effective configuration;
- secret-management responses after a value is saved.

Destination Profile previews show endpoints, TLS and other non-sensitive exporter settings, but mask credential fields and literal header values. A `${secret:key}` reference remains visible so reviewers can verify which managed key will be used.

The built-in store is an encrypted FleetAMP secret store. It is not a HashiCorp Vault integration. A provider interface for external secret managers can be added later without changing the configuration reference syntax.
