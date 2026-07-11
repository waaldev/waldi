# Roadmap

What's shipped, in progress, planned, and deliberately never built. Reflects the maintainer's current thinking, not a commitment or timeline.

Each item notes the side of the product it serves (**Reader**, **Writer**, **Both**, or **Infra**) and priority (**P1** highest).

## Shipped

| Feature | Side | Priority | Notes |
|---|---|---|---|
| Wildcard "not for me" skip + language filter | Reader | P1 | Skip endpoint, reader↔post language matching, impression floor, admin-curated pool via Telegram (short/test posts excluded). |
| Writer digest email (stats + letters) | Writer | P1 | Stats-as-narrative + new letters, delivered each morning. |
| Reader digest email (new posts + today's stranger) | Reader | P1 | New posts from followed writers + the daily wildcard, delivered each morning to registered readers and to captured emails alike. |
| Open reader signup (writing stays invite-only) | Reader | P1 | Reading accounts are open to anyone (email + password). Writing still requires an invite — `/write` checks `CanWrite` and a redeemed write-invite code. |
| Email capture for anonymous visitors | Reader | P1 | One field on public post pages captures an email for the digest, no account needed; adopted automatically if that email later signs up. |
| Letters abuse guard | Both | P1 | Letters require a few completed readings first, plus a daily send limit. A report link on individual letters is still open — see Next. |
| RSS full-content feeds | Writer | P1 | Full-content `/feed.xml`, `<link rel=alternate>` autodiscovery in the blog head, and a footer RSS link. |
| Loop KPI instrumentation | Infra | P1 | Tracks % posts reaching the impression floor within 24h, % earning a follow/letter, time-to-first-follower, week-4 writer retention, reader ritual rate, wildcard completion/skip/follow-conversion. Rising skip-rate is treated as a fire alarm. |
| Pages (About / Now) in blog nav | Writer | P2 | `type=page` posts managed in Settings, shown in blog header, never in feeds. |

## Next

| Feature | Side | Priority | Notes |
|---|---|---|---|
| Report link on letters | Both | P1 | The remaining piece of the letters abuse guard — a report action on individual letters, surfaced to admins. |
| Public reading, anonymous wildcard | Reader | P2 | Blogs and RSS are already public to logged-out visitors. What's left: the logged-out landing page still shows two static sample posts instead of a live wildcard. |
| Blogfa / Persianblog / WordPress import | Writer | P2 | The blog.ir importer shipped and proved the pattern (CLI + hidden settings page); editable publish dates shipped. Remaining: Blogfa, Persianblog, WordPress. |
| Export (zip of posts) | Writer | P2 | JSON export of all posts already exists. Remaining: the promised Markdown + HTML zip. |

## Idea

| Feature | Side | Priority | Notes |
|---|---|---|---|
| Abuse handling basics | Infra | P2 | A general report path + admin actions through the existing Telegram bot, beyond the letters-specific guard above. Second gate before going fully public. |
| Waitlist for non-invited signups | Both | P2 | One email field on the landing page; captures demand while writing stays invite-only. |
| Scoring & tier escalation (100 → 500 → 2000) | Infra | P3 | `0.6 × follow-rate + 0.3 × completion-rate + 0.1 × letter-rate`, Wilson-adjusted. Replaces manual wildcard-pool curation once volume demands it — see the [Wildcard selection](README.md#wildcard-selection) section of the README. |
| Owner SSO on custom domains (one-time token handoff) | Writer | P3 | Mint a local session for the domain owner only, via a 60-second single-use token, avoiding cross-domain cookie tricks. Ship only if the no-owner-buttons-on-custom-domains default actually annoys writers. |

## Never (by principle)

These aren't backlog — they're explicitly ruled out by the product's own rules.

| Feature | Side | Why not |
|---|---|---|
| Comments | Both | Violates the "letters, not comments" principle — public reply threads produce performative dynamics the product is built to avoid. |
| Themes / customization | Writer | Violates "constraint is the feature" — uniform presentation is deliberate, not a missing feature. |
| Public metrics / leaderboards | Both | Violates "no public metrics, anywhere, ever" — all stats are private to the writer. |
