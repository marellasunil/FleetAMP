# FleetAMP E2E Test and Presentation Plan

## Evidence rules

- Capture the Kubernetes status before testing.
- Capture each important FleetAMP page after the action completes.
- Record the expected result, actual result and pass/fail outcome.
- Do not mark an upcoming UI control as implemented.
- Keep credentials and tokens out of screenshots and terminal recordings.

## Test matrix

| ID | Release | Test | Expected evidence |
| --- | --- | --- | --- |
| 01 | v0.1.0 | First-time setup and login | FleetAMP dashboard |
| 02 | v0.1.0 | Three collector scenarios connect | Fleet inventory with distinct identities |
| 03 | v0.1.0 | First configuration reaches `collector-new` | Healthy agent and effective configuration |
| 04 | v0.2.0 | Create ownership group and assign collectors | Group details |
| 05 | v0.2.0 | Target by label | Target preview |
| 06 | v0.2.0 | Create and validate a version | Validation result and diff |
| 07 | v0.2.0 | Request and approve deployment | Approval record |
| 08 | v0.2.0 | Verify deployment | Deployment status and effective configuration |
| 09 | v0.2.0 | Introduce controlled drift | Agent drift details and group drift count |
| 10 | v0.2.0 | Roll back | Previous version restored |
| 11 | v0.3.0 | Import existing effective configuration | Parsed component inventory |
| 12 | v0.3.0 | Apply safe baseline | Before/after YAML |
| 13 | v0.3.0 | Review collector adoption | Readiness checks |
| 14 | v0.3.0 | Match governed Pattern | Score and evidence |
| 15 | v0.3.0 | Adopt or retain custom configuration | Change preview |
| 16 | v0.3.0 | Review fleet-wide adoption | Adoption inventory and filters |

## Screenshot checklist

1. Namespace workloads and image versions
2. FleetAMP setup or login
3. Fleet inventory
4. Collector detail for each scenario
5. Group membership and labels
6. Configuration editor and validation
7. Approval request and decision
8. Successful deployment
9. Effective configuration
10. Drift summary and details
11. Rollback result
12. Migration import
13. Standardization comparison
14. Adoption readiness
15. Pattern matching
16. Fleet-wide adoption

## Presentation outline

1. FleetAMP and the tested release scope
2. Kubernetes lab architecture
3. Collector scenarios
4. v0.1.0 results
5. v0.2.0 governance lifecycle
6. Deployment, drift and rollback
7. v0.3.0 migration workflow
8. Pattern matching and adoption
9. Fleet-wide assessment
10. Findings, limitations and next work

## Video chapters

1. Lab purpose and isolation
2. Kubernetes deployment
3. FleetAMP setup
4. Collector enrollment
5. v0.2.0 governance regression
6. v0.3.0 migration and adoption
7. Results and cleanup
