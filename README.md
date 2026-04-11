# misty
Backend application for [Misty](https://apps.apple.com/nl/app/misty-fog-forecasts/id6761374118), an iOS app I created mainly for myself. I like to do photography in foggy conditions, but always find out too late. Existing weather apps can give you forecasts, but do not send you push notifications ahead of time. Misty will do exactly that.

## Design

```mermaid
graph LR
    iOS[iOS app] -- HTTPS --> CF[Cloudflare<br/>WAF · rate limiting]

    subgraph Hetzner VPS
        TS[tailscaled]
        subgraph frontend network
            Caddy[Caddy<br/>mTLS · rate limiting]
            App[misty<br/>Go app]
        end
        subgraph backend network
            App
            PG[(PostgreSQL)]
        end
    end

    CF -- mTLS --> Caddy
    Caddy -- HTTP --> App
    App -- pgx --> PG
    App -. forecast .-> OM[Open-Meteo]
    App -. push .-> APNs[Apple APNs]
    GH[GitHub Actions] -. Tailscale SSH .-> TS
```
