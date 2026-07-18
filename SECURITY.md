# Security Policy

Waldi is run by one person. Response times are best-effort. I fix things quickly when I can.

## Reporting a vulnerability

Do not open a public GitHub issue for a security vulnerability. That's reckless. Email **me@aminzamani.com** instead.

Include exactly this:
- What the issue is and what it breaks.
- Steps to reproduce it. (A minimal example is gold.)
- Any relevant logs, requests, or screenshots.

You will get a human acknowledgement within a few days. If the issue is real, I will prioritize a fix. You will be credited in the release notes unless you ask to remain anonymous.

## Scope

This covers the `waldi` codebase. The Go application, the templates, and the editor JS. It entirely ignores third-party services like Cloudflare, Resend, or your hosting provider. If you find a hole in their infrastructure, tell them.
