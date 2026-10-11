# FleetAMP documentation

This index separates operator instructions, user workflows, release information
and contributor verification. Start with the section that matches your task.

## Start here

| Goal | Documentation |
| --- | --- |
| Understand FleetAMP and its architecture | [Project overview](../README.md) |
| Install FleetAMP as a Linux service | [systemd deployment](../deploy/systemd/README.md) |
| Run the local FleetAMP and Collector lab | [Compose lab](../deploy/compose/README.md) |
| Understand the console navigation and roles | [Scalable console](scalable-console.md) |
| Recover or change a password | [Password management and recovery](password-recovery.md) |

## v0.3.0 workflows

| Workflow | Documentation |
| --- | --- |
| Review the v0.3.0 scope and limitations | [v0.3.0 release notes](releases/v0.3.0.md) |
| Choose an instrumentation approach and generate governed configuration | [Guides and Blueprints](blueprints.md) |
| Import an existing Collector configuration | [Configuration migration](migration.md) |
| Store group-scoped secret references safely | [Group secrets](group-secrets.md) |
| Configure and operate governed repository delivery | [Governed GitOps delivery](gitops.md) |
| Verify the source, binary, database migration and optional image | [Local release verification](testing-v0.3.0.md) |
| Validate the complete UI and governance journeys | [v0.3.0 E2E checklist](e2e-v0.3.0.md) |

## Release documentation

| Release | Focus |
| --- | --- |
| [v0.1.0](releases/v0.1.0.md) | Initial community preview |
| [v0.2.0](releases/v0.2.0.md) | Governed fleet management |
| [v0.3.0](releases/v0.3.0.md) | Guides, migration, component lifecycle and governed GitOps |

## Recommended v0.3.0 release sequence

1. Create a clean release-candidate branch from `main`.
2. Run the [local release verification](testing-v0.3.0.md).
3. Test an upgrade using a copy of a v0.2 database.
4. Run the [manual E2E checklist](e2e-v0.3.0.md).
5. Fix only release-blocking defects and rerun the affected checks.
6. Build the Docker image and release archives.
7. Merge the release PR, tag `v0.3.0`, and publish the release notes and checksums.

The automated verification deliberately disables real Git provider writes. Use
a dedicated test repository when exercising GitHub pull-request delivery.
