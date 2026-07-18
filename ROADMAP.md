# Roadmap

What we shipped. What we're building. And the things we will absolutely never build.

This isn't a corporate promise or a timeline. It is a reflection of my current thinking as the sole maintainer. I rank every item by priority (**P1** is highest) and flag who it serves (**Reader**, **Writer**, **Both**, or **Infra**).

## Shipped

| Feature | Side | Priority | Notes |
|---|---|---|---|
| Wildcard "not for me" skip + language filter | Reader | P1 | Skip endpoint, reader↔post language matching, and an impression floor. Curated via Telegram. (Short or test posts are banished.) |
| Writer digest email (stats + letters) | Writer | P1 | Stats framed as a quiet morning narrative. Paired with your unread letters. |
| Reader digest email (new posts + today's stranger) | Reader | P1 | Your followed writers plus the daily wildcard. Sent to registered readers and captured emails alike. |
| Open reader signup (writing stays invite-only) | Reader | P1 | Anyone can create a reading account. Writing demands an invite. The `/write` endpoint enforces `CanWrite` and a redeemed code. |
| Email capture for anonymous visitors | Reader | P1 | One field on public posts to capture emails. No account required. Seamlessly upgrades if they register later. |
| Letters abuse guard | Both | P1 | You can't just spam letters. You need a history of completed readings. Plus a daily throttle. |
| RSS full-content feeds | Writer | P1 | Full-content `/feed.xml`. Built-in `<link rel=alternate>` autodiscovery. We don't cripple RSS. |
| Loop KPI instrumentation | Infra | P1 | Tracks the core loop: time-to-first-follower, week-4 writer retention, and wildcard skip-rates. A rising skip-rate triggers a fire alarm. |
| Pages (About / Now) in blog nav | Writer | P2 | Pure `type=page` posts. Managed in Settings. Hidden from feeds. |

## Next

| Feature | Side | Priority | Notes |
|---|---|---|---|
| Report link on letters | Both | P1 | The final piece of the abuse guard. A simple report action on any letter, directly surfaced to admins. |
| Public reading, anonymous wildcard | Reader | P2 | Blogs and RSS are already open. We need to swap the two static landing page posts for a live wildcard. |
| Blogfa / Persianblog / WordPress import | Writer | P2 | The blog.ir importer proved the pattern. The remaining giants are next. |
| Export (zip of posts) | Writer | P2 | JSON export already works. The promised Markdown + HTML zip export is coming. |

## Idea

| Feature | Side | Priority | Notes |
|---|---|---|---|
| Abuse handling basics | Infra | P2 | A generalized reporting path feeding into the Telegram bot. This is the final gate before going completely public. |
| Waitlist for non-invited signups | Both | P2 | A single email field on the landing page. It captures demand while we protect the writer culture. |
| Scoring & tier escalation (100 → 500 → 2000) | Infra | P3 | Replaces manual curation with math. `0.6 × follow-rate + 0.3 × completion-rate + 0.1 × letter-rate`. Wilson-adjusted. See the [Wildcard selection](README.md#wildcard-selection) section. |
| Owner SSO on custom domains | Writer | P3 | Minting a 60-second single-use token to hand off a local session to a custom domain. Avoids the nightmare of cross-domain cookies. |

## Never (by principle)

This isn't a backlog. These features are ruled out by the product's foundational principles.

| Feature | Side | Why not |
|---|---|---|
| Comments | Both | "Letters, not comments." Public reply threads breed performative arguments. We built Waldi to kill that dynamic. |
| Themes / customization | Writer | Constraint is a feature. We enforce a uniform, considered presentation so writers can just write. |
| Public metrics / leaderboards | Both | "No public metrics, anywhere, ever." We don't rank humans. Your stats are yours alone. |
