---
sidebar_position: 2
title: Quick Start
---

# Quick Start

FleetAMP is currently in early development. This page documents the developer workflow rather than promising a production installation path that does not yet exist.

## Prerequisites

- Go version required by the repository `go.mod`
- Git

## Build from source

```bash
git clone https://github.com/marellasunil/FleetAMP.git
cd FleetAMP
go test ./...
go vet ./...
go run ./cmd/fleetamp
```

The current service exposes its development health endpoint. OpAMP onboarding instructions will be added when the OpAMP adapter is implemented and verified against a real Supervisor.

## Documentation locally

```bash
cd website
npm install
npm start
```

Docusaurus starts a local development server and reloads documentation changes automatically.
