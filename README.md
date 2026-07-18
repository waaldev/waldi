# Waldi

A quiet place to write, and to be read.

Live at **[waldi.blog](https://waldi.blog)**. Reading is open to everyone. Writing requires an invitation for now. because I can curate a minimum quality of wildcard posts.

![A Waldi post in light and dark mode](docs/screenshot.jpg)

Waldi is a blogging platform, but not another minimalistic blogging engine. every post delivers to at least 100 readers before any black-box algorithm judges it, and every registered reader will get a random stranger post each morning in their inbox. I use a concept called [Serendipity](https://amin.waldi.blog/serendipity).

## Features

- **Custom domains** — point your own domain at your blog.
- **A modern editor** — Tiptap-based, WYSIWYG.
- **Letters** — private replies from readers.
- **RSS** — full-content feeds, autodiscoverable.
- **Bilingual by design** — English and Persian (RTL) out of the box, per-post language.
- **Blog importers** — more platforms on the way.
- **Self-hostable** — MIT licensed, run your own instance.

## Quickstart

```bash
cp .env.example .env
make db        # spin up Postgres and MinIO
make migrate   # run migrations
make dev       # parallel run: air (Go live reload) + esbuild --watch
```

Use `waldi.test` (with `/etc/hosts` entries) to test cross-subdomain cookie logic.

## Contributing

See `CONTRIBUTING.md`. Day-to-day works lands in `develop`. `main` is production.

## Roadmap

Check `ROADMAP.md`

## License

MIT — see `LICENSE`.
