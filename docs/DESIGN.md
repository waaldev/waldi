# Waldi — Design Guidelines

*The feeling to build: a well-loved book read by lamplight. Cozy comes from warmth and softness; minimal comes from discipline; lovely comes from a handful of small, human moments placed exactly right.*

Everything below is derived from one test: **would this feel at home printed on good paper?** If a UI element couldn't exist in a beautifully typeset book — a card shadow, a badge, a spinner — it has to justify itself or go.

---

## 1. Design Tokens

Define these as CSS custom properties in `main.css`. Every rule in the stylesheet derives from them; no raw hex values anywhere else.

### Color — "ink on paper, by lamplight"

Light mode:
```css
--paper:        #FAF7F2;  /* warm paper — cream, not beige, not white */
--paper-raised: #F3EFE7;  /* input fields, the wildcard's gentle tint  */
--ink:          #26241F;  /* warm near-black, never #000               */
--ink-soft:     #6B655A;  /* secondary text: names, dates, counts      */
--ink-faint:    #A39C8E;  /* placeholders, disabled, hairlines         */
--accent:       #3D5A80;  /* fountain-pen blue: links, follow, focus   */
--accent-ink:   #2C4361;  /* accent on hover / active                  */
```

Dark mode ("the reading lamp," not "developer terminal"):
```css
--paper:        #201E1B;  /* warm dark brown-grey, never blue-black    */
--paper-raised: #292623;
--ink:          #EAE5DC;
--ink-soft:     #9C958A;
--ink-faint:    #5E594F;
--accent:       #8FB0D9;  /* the same ink, moonlit                     */
```

Rules:
- **The accent appears in at most one or two places per screen.** Links inside prose, the Follow button, focus rings. If a screen shows accent in three places, remove one.
- **No other hues exist.** No success-green, no error-red badges. Errors and confirmations are written in words, in ink (see §7). One accent is what makes the palette feel like a considered object rather than an app theme.
- The fountain-pen blue is a deliberate choice over the fashionable terracotta/clay accent every "warm minimal" site uses right now — ink is Waldi's material, and it keeps us out of that visual cliché.

### Typography — the actual identity

| Role | Face | Notes |
|---|---|---|
| Body / reading | **Newsreader** (or Literata as fallback choice) | A serif drawn for screen reading with real warmth. Use its optical sizes: text weight for body, display cuts for titles. |
| UI chrome | **system sans** (`-apple-system, Segoe UI, ...`) | Deliberately quiet. UI text should feel like the room, not the furniture. Nav, buttons, labels, stats. |
| Never | a monospace, a geometric display face, a third family | Two voices: the book (serif) and the margin notes (sans). |

Type scale (rem, 1rem = 16px):
```
--text-title:   1.75rem / 1.25   (post titles on their own page, Newsreader 500)
--text-feed:    1.25rem / 1.35   (titles in the feed list, Newsreader 500)
--text-body:    1.1875rem / 1.65 (19px reading text, Newsreader 400)
--text-ui:      0.9375rem / 1.4  (15px, sans — buttons, nav, letters UI)
--text-meta:    0.8125rem / 1.4  (13px, sans — dates, counts, labels)
```

Reading measure: **max-width 34rem** (~65 characters at 19px) for all prose, centered. This one number is most of the "book" feeling. Paragraphs separated by space (0.75em), no indents, no justification (ragged right is friendlier on screens). Blockquotes: italic, small left inset, a 2px `--ink-faint` rule — no giant quotation marks.

### Spacing & shape

- Spacing scale: 4 / 8 / 12 / 20 / 32 / 52 / 84px (a loose golden-ish ramp — generous jumps are what read as "calm").
- **Radius: 6px** on the few boxes that exist (inputs, buttons). Not 0 (severe), not 16 (app-like).
- **No shadows anywhere.** Elevation doesn't exist in a book. Layering is done with `--paper-raised` and space.
- **Hairlines only when space can't do the job:** 1px `--ink-faint` at 40% opacity. Feed items are separated by whitespace alone.

---

## 2. The Signature: ※ and the ritual moments

Every memorable product has one recurring mark. Waldi's is the **reference mark ※ (komejirushi)** — a typographic character, from the world of print, unfamiliar enough to feel discovered. It appears in exactly three places:

1. **Beside "Today's stranger"** in the feed — the wildcard's only distinguishing decoration, set in `--accent`.
2. **As the end-of-post mark** — where old books put ❦, every Waldi post ends with a small centered ※ in `--ink-faint`. The signal that you truly finished.
3. **In the logo/wordmark** — `waldi ※` or the mark alone as favicon.

It costs nothing, renders everywhere, and after a week of use, ※ *means* "a small gift of writing." That's the lovely part — an owned symbol instead of an icon set.

**The finish line** (the other ritual): when the feed is done, the list simply ends with generous space and one centered serif line — "That's everything for today. ※" — with a rotating second line beneath in `--ink-soft` (see §7 for voice). No illustration, no mascot, no confetti. The restraint *is* the coziness: it treats the reader as an adult who is now free to go.

---

## 3. Screen-by-Screen Guidance

### Home (the reading inbox)
- One centered column, same 34rem measure as prose — the feed is typography, not a component list.
- Each item: writer's name (`--text-meta`, sans, `--ink-soft`) above the title (`--text-feed`, serif, `--ink`) above 1½ lines of the opening (`--text-ui` size but in the serif, `--ink-soft`, ellipsis). ~52px space between items. Nothing else — no thumbnails, no read-time, no dot-separated metadata rows.
- The whole item is the link. Hover: title shifts to `--accent-ink`, nothing moves.
- The wildcard sits at top on `--paper-raised` with soft padding, labeled `※ Today's stranger` in `--text-meta` accent. Beneath the excerpt, one quiet text action: "not for me" (`--ink-faint`, no border, no icon).
- Day groupings, if shown at all: a lone small-caps sans label ("today") — no rules, no pills.

### Reading view
- Nothing above the title except a small back-arrow "waldi ※" wordmark and the writer's name. **No sticky header** — when you scroll into the text, the interface leaves the room.
- Title in display Newsreader, then the writer + date line in `--text-meta`, then 52px of air, then prose.
- Images: full column width, 6px radius, optional caption in `--text-meta` italic centered. Dividers (the writer's `+` divider) render as a centered `※` — the mark again, doing real work.
- At the end: the closing ※, then 84px of air, then the action row — **Follow · Write a letter · Share** — set as text buttons in the sans, Follow alone carrying the accent (solid accent bg, paper text, 6px radius; it's the single most important button in the product and the only solid button on the screen). After following, the button becomes quiet text: "Following ✓".
- Absolutely no numbers on this screen. Ever.

### The editor
- The chrome disappears: paper background edge-to-edge, a tiny top bar with only "Drafts" left, "Publish" right (ghost button until the doc has a title and >0 words, then solid accent).
- Title is a serif input styled identically to a rendered title, placeholder "Title" in `--ink-faint`. Body placeholder: "Write."
- The floating toolbar (on text selection): a small `--ink` pill, paper-colored icons/labels, 6px radius — the one dark element in light mode, intentionally, like a pen. Bold · Italic · Link · H2 · Quote.
- Autosave indicator: the word "Saved" in `--ink-faint` at top center, fading in and out. Never a spinner.
- **WYSIWYG must mean it:** the editor uses the exact same type tokens as the reading view. Zero difference between writing and reading except the caret.
- On publish: a full-screen paper moment — "Published. ※ Your post is on its way to 100 readers." One serif sentence, one "View post" link. This is the writer's ritual; give it a full breath, not a toast.

### Letters
- The composer opens as its own page (never a modal — letters deserve a room). Recipient line: "To [writer], about *[post title]*" in `--text-meta`. Body in the serif at reading size — you write a letter in the same type you read one.
- Send button reads **"Send letter"**, and the confirmation: "Sent. Letters travel quietly — you'll hear back if they write."
- The inbox lists letters like the feed lists posts: sender, first line, space. Reading a letter uses the reading view's typography wholesale.

### Writer's mailbox (stats)
- No charts, no tiles, no grids. Per post, a short serif paragraph: "142 people read this. 89 finished. 6 followed you because of it. 2 wrote to you." Numbers set slightly heavier (500) in the same ink — emphasis, not decoration.
- Updated daily; say so plainly: "Numbers update each morning." (This sentence prevents refresh-checking better than any design trick.)

---

## 4. Motion

Three animations exist in the entire product:

1. **Page content fade-up on load:** 200ms, 4px rise, ease-out. Every page, uniformly — the "turning a page" feel.
2. **The Saved indicator** fading in/out (300ms).
3. **The publish moment:** the confirmation sentence fades in over 400ms with a slight settle.

Everything else is instant. Hovers change color with a 120ms transition, nothing moves or grows. `prefers-reduced-motion` disables 1 and 3 outright. No skeleton screens — server-rendered pages arrive whole; if something is slow, fix the query, not the perception.

---

## 5. Interaction Details (where "lovely" actually lives)

- **Focus rings:** 2px `--accent` outline with 2px offset, on everything, always visible. Keyboard users get the same considered product.
- **Link style in prose:** `--accent` text with a subtle underline (`text-decoration-thickness: 1px; text-underline-offset: 3px`). Links in UI chrome: no underline until hover.
- **Selection color:** `--accent` at ~20% opacity — even selecting text feels on-palette.
- **Buttons:** one solid style (accent bg) reserved for the primary act of a screen (Follow, Publish, Send letter). Everything else is text or ghost. Two button styles total in the product.
- **Forms (login/signup):** labels above inputs in `--text-meta`, inputs on `--paper-raised` with 1px faint border, generous 12px padding. The signup page should be as considered as the reading page — it's the first impression.
- **The favicon and touch icon:** the ※ mark in ink on paper. Details like this are half of "lovely."
- **Print stylesheet** for posts: hide all chrome, black on white, serif. A blogging platform whose posts print beautifully is a quiet statement of values. It's ~15 lines of CSS.

---

## 6. Layout & Responsiveness

- The product is **one column at every size.** Desktop gets more margin, not more columns. This makes responsive design nearly free and enforces calm.
- Base padding: 20px side margins on mobile, growing to auto-centered 34rem column above 600px.
- Tap targets ≥ 44px even where the visual text is small (pad the link, not the type).
- The editor toolbar on mobile: docks above the keyboard instead of floating.
- Test the reading view at 360px and at 1440px early — if it's beautiful at both with the same CSS, the system is right.

---

## 7. Voice & Microcopy

The interface speaks like a considerate librarian: warm, brief, plain verbs, sentence case, no exclamation marks, no emoji, never cute-clever at the cost of clarity.

| Moment | Copy |
|---|---|
| Empty feed (new reader) | "Your feed is quiet. Follow a writer, or meet today's stranger below. ※" |
| Finish line | "That's everything for today. ※" + rotating line: "Go take a walk." / "See you tomorrow." / "The strangers will keep writing." |
| Empty drafts | "Nothing here yet. Write something." |
| Publish confirm | "Published. ※ Your post is on its way to 100 readers." |
| Letter sent | "Sent. Letters travel quietly — you'll hear back if they write." |
| First follower notification (digest) | "Someone followed you yesterday. They found you through *[post]*." |
| Error (form) | Plain ink sentence under the field: "That username is taken." Never red, never "Oops." |
| 404 | "There's no page here. ※" + link home |
| Wildcard skip | button simply reads "not for me" — lowercase, no confirmation |

Consistency rule: an action keeps its name through the whole flow — the button says "Publish," the confirmation says "Published," the list says "Published yesterday."

---

## 8. Persian & RTL

Waldi is bilingual by design: direction is a property of a **post**, not the site. The one-column layout makes this nearly free.

### Typefaces (mirroring the Latin pairing)

| Role | Persian face | Latin counterpart |
|---|---|---|
| Body / reading | **Markazi Text** (Borna Izadpanah; warm Naskh flavor, drawn for continuous screen reading) | Newsreader |
| UI chrome | **Vazirmatn** (variable, neutral, excellent Latin support) | system sans |
| Fallback body | Noto Naskh Arabic | Literata |

Self-host all fonts — readers may not have reliable access to Google's CDN, and it's faster regardless. Font stacks list both scripts' faces so mixed-language text falls through correctly:

```css
--font-serif: "Newsreader", "Markazi Text", Georgia, serif;
--font-sans:  -apple-system, "Segoe UI", "Vazirmatn", sans-serif;
```

### Rules

- **Per-post direction:** store `lang` + `dir` on each post (auto-detected from content); set them on the `<article>`. The feed mixes directions naturally; feed items inherit the direction of their own post.
- **Persian metrics:** at equal px sizes Persian reads smaller and denser. For `:lang(fa)` prose: body ~1.3125rem (21px), line-height 1.9. Same 34rem measure.
- **Never letter-space Persian** — it breaks the connected script. Any style using `letter-spacing` (small-caps labels) gets a `:lang(fa)` exception resetting it to 0.
- **Digits:** Persian digits (۱۲۳) inside Persian prose; Latin digits in UI chrome and stats.
- **Logical properties everywhere:** `margin-inline-start`, `padding-inline`, `text-align: start` — never left/right — so RTL costs zero extra CSS.
- **The ※ signature is script-neutral** and unchanged. "Today's stranger" ⇄ «غریبه‌ی امروز ※».
- Blockquote inset rule, letter composer, editor — all inherit direction from content; the toolbar and chrome stay in the UI language.

---

## 9. What Is Banned (the discipline list)

No card shadows · no borders where space works · no badges or pills · no red/green/yellow status colors · no icons where a word fits (the product needs maybe five icons total) · no spinners or skeletons · no toasts (confirmations happen in place, in words) · no modals except image zoom · no sticky headers on reading surfaces · no thumbnails in feeds · no visible metrics on any reading surface · no emoji in UI copy · no third typeface · no infinite scroll, anywhere, ever.

When a new feature arrives, it must be dressed in the tokens above and pass the book test. If it can't, the feature — not the design system — is probably wrong.
