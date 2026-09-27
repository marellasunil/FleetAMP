# FleetAMP governance and licensing

FleetAMP is an independent, vendor-neutral community project. It is not an
official OpenTelemetry project and does not grant special authority over the
OpenTelemetry specification or upstream implementations.

## Project direction

The project is currently maintainer-led. Maintainers set release scope,
security boundaries and compatibility expectations after considering public
issues, design proposals and pull-request feedback. Decisions affecting data
integrity, authorization, approval separation or protocol compatibility require
tests and written rationale.

Small fixes can proceed directly through a focused pull request. Contributors
should open a design issue before proposing a new subsystem, persistence model,
public API, managed-agent adapter or incompatible behavior.

## Roles

- **Users** evaluate FleetAMP and report operational experience.
- **Contributors** submit documentation, tests, fixes or proposals.
- **Reviewers** provide recurring technical review in an area of the project.
- **Maintainers** merge changes, manage releases and make final scope and
  security decisions.

These roles describe participation rather than employment, support entitlement
or ownership of the FleetAMP name.

## Licensing

FleetAMP source code and repository documentation are currently licensed under
the [Apache License 2.0](LICENSE). It permits use, modification, distribution,
forking and commercial use, subject to its copyright, license and notice terms.
The license does not require modified versions or hosted services to publish
their source code.

Open source cannot allow contribution while forbidding all forks. If the
project later decides that distributed derivatives or hosted modifications must
remain open, a copyleft license such as GPLv3 or AGPLv3 would need a separate,
explicit governance and legal review. No such change is made by this document.
A future relicensing proposal must address contributor consent, existing
releases, dependencies, trademarks and the treatment of inbound contributions.

Contributions accepted under the current model are provided under Apache-2.0.
No Contributor License Agreement is currently required.

## Name and compatibility claims

The Apache license covers copyright permissions; it does not grant rights to
misrepresent a fork as an official FleetAMP release. Forks should use a distinct
name when their behavior or compatibility differs materially and must not imply
endorsement by FleetAMP maintainers or the OpenTelemetry project.

## Releases and support

Pre-1.0 releases are community previews. A release documents its tested
platforms, known limitations and upgrade notes, but does not create a commercial
support commitment. Security reporting follows [SECURITY.md](SECURITY.md).
