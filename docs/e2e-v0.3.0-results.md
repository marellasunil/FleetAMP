# FleetAMP v0.3.0 E2E evidence report

Use this report while completing the [v0.3.0 E2E checklist](e2e-v0.3.0.md).
Replace each `Not run` value and add links to sanitized screenshots as the test
progresses. Do not include credentials, secret values, private repository URLs,
session cookies or personal data.

## Test run

| Field | Value |
| --- | --- |
| Date | |
| Tester | |
| Commit | |
| Image or binary | |
| FleetAMP version | `v0.3.0-rc1` |
| Kubernetes namespace | |
| Database source | Fresh / copied v0.2 |
| Git provider mode | Dry-run / GitHub PR |
| Overall result | Not run |

## Environment readiness

| Check | Result | Evidence or notes |
| --- | --- | --- |
| Pod is `1/1 Running` | Not run | |
| Port-forward is active | Not run | |
| `/health` returns success | Not run | |
| `/ready` returns success | Not run | |
| Final commit matches the deployed build | Not run | |

## Scenario results

| ID | Scenario | Result | Screenshot or notes |
| --- | --- | --- | --- |
| 01 | Administrator sign-in and password change | Not run | |
| 02 | Group Owner authorization boundaries | Not run | |
| 03 | Configuration version, approval and deployment | Not run | |
| 04 | Drift detection and audit evidence | Not run | |
| 05 | Migration parse, normalize, validate and save | Not run | |
| 06 | Progressive Guide decision path | Not run | |
| 07 | Blueprint generation and validation | Not run | |
| 08 | Group secret remains write-only | Not run | |
| 09 | Repository connection creation and validation | Not run | |
| 10 | Lifecycle proposal and target resolution | Not run | |
| 11 | Proposal approval separation of duties | Not run | |
| 12 | Immutable GitOps preview and execution | Not run | |
| 13 | Optional GitHub pull-request delivery | Not run / N/A | |
| 14 | Integration and execution statistics | Not run | |
| 15 | GitOps and Integrations audit filtering | Not run | |
| 16 | Failure and recovery scenarios | Not run | |
| 17 | Invalid `agents.json` startup recovery | Not run | |
| 18 | Restart and database persistence | Not run | |

Allowed result values are `Pass`, `Fail`, `Blocked` and `N/A`. Explain every
`Fail`, `Blocked` or `N/A` result in the observations section.

## Screenshot index

Store screenshots in `docs/assets/v0.3.0/e2e/`. After adding a screenshot,
replace the example text below with a Markdown image and a short description of
what it proves.

```markdown
![Migration preview showing validated configuration](assets/v0.3.0/e2e/05-migration-preview-valid.png)

Evidence: the normalized configuration passed validation before an immutable
version was saved.
```

Prefer one screenshot per important state transition. Crop unrelated desktop
content and redact tokens, passwords, cookies, email addresses, private
repository URLs and secret values before committing an image.

## Observations and defects

| ID | Severity | Observation or defect | Resolution or issue |
| --- | --- | --- | --- |
| | | | |

## Release decision

| Gate | Result |
| --- | --- |
| Automated verifier passed on final commit | Not run |
| Required E2E scenarios passed | Not run |
| Database migration and restart passed | Not run |
| No credentials exposed in evidence | Not run |
| Release-blocking defects resolved | Not run |
| Documentation matches tested behavior | Not run |

**Decision:** Not ready

**Approved commit:**

**Known limitations accepted for v0.3.0:**

**Tester sign-off:**

The `v0.3.0` tag must point to the approved commit recorded above.
