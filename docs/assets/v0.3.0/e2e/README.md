# v0.3.0 E2E screenshots

Place sanitized screenshots captured during the v0.3.0 E2E run in this
directory. Use sequential, descriptive filenames:

```text
01-auth-admin-signed-in.png
02-auth-group-owner-scope.png
03-configuration-approved.png
04-drift-detected.png
05-migration-preview-valid.png
06-guide-progressive-path.png
07-blueprint-generated.png
08-secret-reference-masked.png
09-integration-validated.png
10-lifecycle-targets-resolved.png
11-lifecycle-approved.png
12-gitops-execution-completed.png
13-github-pull-request.png
14-statistics-summary.png
15-audit-gitops-filter.png
16-failure-recovery.png
17-agent-snapshot-recovered.png
18-restart-persistence.png
```

The filename list is a guide, not a requirement to capture redundant images.
Each committed screenshot must support a result in
`docs/e2e-v0.3.0-results.md`.

Before committing, crop unrelated desktop content and redact credentials,
secret values, session cookies, email addresses and private repository URLs.
