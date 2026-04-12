<p align="center">
  <img src="assets/misty_fat-iOS-Default-1024x1024@1x.png" width="128" alt="Misty app icon" />
</p>

# Misty
Backend application for [Misty](https://apps.apple.com/nl/app/misty-fog-forecasts/id6761374118), an iOS app I created mainly for myself. I like to do photography in foggy conditions, but always find out too late. Existing weather apps can give you forecasts, but do not send you push notifications ahead of time. Misty will do exactly that.

## Architecture

```mermaid
graph LR
    iOS[iOS app] -- HTTPS --> CF[Cloudflare<br/>WAF · rate limiting]

    subgraph VPS
        TS[tailscaled]
        Caddy[Caddy<br/>mTLS · rate limiting]
        App[Misty<br/>Go app]
        PG[(PostgreSQL)]
        TS ~~~ Caddy
    end

    CF -- mTLS --> Caddy
    Caddy -- HTTP --> App
    App -- pgx --> PG
    App -. forecast .-> OM[Open-Meteo]
    App -. push .-> APNs[Apple APNs]
    GH[GitHub Actions] -. SSH .-> TS
```

## Application

One goroutine serves the API, another refreshes forecasts. The refresh loop `select`s on an hourly ticker and a buffered channel. Creating a monitor pushes it onto that channel so it gets its first forecast right away.

Each refresh writes forecasts, monitor state, and any fog notifications in a single transaction. Notifications use an outbox: delivery runs as a separate step after each pass, pushing unsent rows to APNs and marking them sent.

```mermaid
sequenceDiagram
    participant App as iOS app
    participant API as API handler
    participant Ch as Channel
    participant Cycle as Refresh loop
    participant OM as Open-Meteo
    participant DB as PostgreSQL
    participant APNs

    alt Timer tick (every hour)
        Cycle->>DB: list active monitors
        DB-->>Cycle: monitors
    else onRefreshNeeded
        App->>API: create/activate monitor
        API->>DB: upsert monitor
        DB-->>API: monitor
        API-->>App: 201 Created
        API-)Ch: non-blocking send
        Ch->>Cycle: monitor
    end

    Cycle->>OM: fetch forecast
    OM-->>Cycle: weather data

    rect rgba(0, 128, 255, 0.05)
        Note over Cycle,DB: single transaction
        Cycle->>DB: save forecasts
        opt fog detected
            Cycle->>DB: insert notification (unsent)
        end
        Cycle->>DB: update monitor state
    end

    rect rgba(255, 165, 0, 0.05)
        Note over Cycle,APNs: outbox delivery
        Cycle->>DB: query unsent notifications
        DB-->>Cycle: pending rows
        Cycle->>APNs: push
        APNs-->>Cycle: accepted
        Cycle->>DB: mark sent
    end
```

## Infrastructure and Deployment

The infrastructure is defined in OpenTofu (`infra/`) and is centered around a Hetzner VPS.

The Hetzner firewall only accepts TCP/443 from Cloudflare IP ranges. There is no public SSH port. Cloudflare sits in front with end-to-end TLS, a managed WAF ruleset and rate limiting at 20 requests per 10 seconds per IP.

The firewall blocks non-Cloudflare traffic, but since Cloudflare is a shared platform, another customer could (theoretically) point their DNS at the origin IP and their requests would pass through. Authenticated Origin Pulls prevent that: `infra/aop.tf` generates a CA and client certificate that only my Cloudflare zone presents. Caddy is configured to `require_and_verify` it, refusing any connection without the right cert.

Caddy adds a second rate limiting layer. It also has a path allowlist, so anything not explicitly routed gets a 404 before it reaches the app.

Deployments run over Tailscale SSH from GitHub Actions using auth keys. This makes the github action runner an ephemeral node in my tailnet for the duration of the deployment. The node is tagged, which makes it subject to an ACL that lets it access only the VPS (and not other resources on my tailnet).
