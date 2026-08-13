# Synology Connector

Synology Connector is the secure GitHub App companion for Zion ReleaseStation, a self-hosted deployment manager for Synology NAS.

It allows each customer to connect their own GitHub account or organization, select the repositories that Zion ReleaseStation may access, and deploy approved code to websites hosted on their Synology system.

## What the connector does

- lets a customer authorize the Synology Connector GitHub App without creating a GitHub App of their own;
- provides Zion ReleaseStation with the list of repositories explicitly granted during GitHub installation;
- reads repository branches, commit history, and release archives for authorized repositories;
- receives verified GitHub push events for automatic deployments;
- routes each event only to the paired ReleaseStation instance and installation that owns it;
- supports private repositories without requiring a personal access token on the Synology NAS.

## How it works

```text
Customer GitHub account or organization
                 │
                 │ GitHub App installation and repository selection
                 ▼
        Synology Connector
                 │
                 │ HTTPS + per-instance credential
                 ▼
        Zion ReleaseStation on Synology
                 │
                 ▼
          Atomic website deployment
```

The Connector is the public integration service. Zion ReleaseStation communicates with it over HTTPS using a credential issued for one NAS instance. GitHub push events are received by the Connector, verified, deduplicated, and queued. The NAS polls its own queue and performs a deployment only when the repository, GitHub installation, branch, and `Push to deploy` setting all match.

## Security and privacy

- The GitHub App private key remains on the Zion Connector service and is never shipped inside the ReleaseStation SPK.
- Customers do not upload a `.pem` file, App ID, client secret, or GitHub personal access token to their NAS.
- Repository access is limited by the repositories selected during GitHub App installation.
- GitHub webhook signatures are validated with `X-Hub-Signature-256` before an event is accepted.
- GitHub delivery IDs are deduplicated so the same event cannot create repeated queue entries.
- Installation tokens are short-lived and are not stored as customer-facing credentials.
- ReleaseStation receives repository metadata and release archive bytes, not the GitHub App private key.
- The Connector does not execute webhook payloads as shell commands.

## Permissions requested

The App should request the minimum permissions required by the installed features:

- **Contents: Read-only** — read authorized repository files and release archives;
- **Metadata: Read-only** — identify repositories and installations.

The App does not need write access to repository contents, Actions, administration, or organization management for the standard ReleaseStation workflow.

## Webhook configuration

For automatic push-to-deploy, configure the GitHub App webhook with:

- **Payload URL:** the public Zion Connector URL followed by `/github/webhook`;
- **Content type:** `application/json`;
- **Secret:** the same value configured on Zion Connector as `CONNECTOR_GITHUB_WEBHOOK_SECRET`;
- **Events:** enable **Push**.

The webhook secret belongs only in the Connector environment. It must not be copied into the Synology SPK, the ReleaseStation UI, a repository, or a customer-facing configuration file.

## Customer experience

The customer installs Synology Connector in the desired GitHub account or organization, selects the repositories to expose, and returns to Zion ReleaseStation. The selected repositories then become available in the site Repository tab. For a site with **Push to deploy** enabled, a push to the configured branch starts the verified atomic deployment workflow automatically.

Synology Connector is designed to keep GitHub authorization centralized and maintainable while keeping every customer's repository selection and ReleaseStation credential isolated from other installations.
