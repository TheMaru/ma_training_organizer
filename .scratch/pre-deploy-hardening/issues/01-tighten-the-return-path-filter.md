# 01 — Tighten the language switcher's return-path filter

Status: ready-for-agent

`returnPath` in `internal/web/locale.go` is the security control for the
client-supplied `return` field of the language switcher. It rejects a value that
does not start with `/`, one starting with `//` or `/\`, and one containing `\r`
or `\n` — but it lets a horizontal tab (`0x09`) through, and browsers strip ASCII
tab and newline from a URL *before* parsing it (WHATWG URL standard). So
`/<TAB>/evil.com` is parsed by the browser as `//evil.com`: a protocol-relative
URL pointing off-site.

The Go side does nothing to stop it, which is worth spelling out because it is
counter-intuitive: `net/url.Parse` rejects any byte below `0x20`, so
`http.Redirect` skips its normalising branch entirely and writes the value
verbatim; `hexEscapeNonASCII` only touches bytes ≥ `utf8.RuneSelf`;
`textproto.TrimString` trims only at the ends of a header value; and a raw HTAB
is legal inside an HTTP/1 field value. The header goes out as
`Location: /<TAB>/evil.com`. The `HX-Redirect` branch of `redirect`
(`internal/web/server.go`) has the same shape, since htmx assigns that header
straight to `location`.

## Why this is a fix and not a vulnerability report

The pre-deployment security review (2026-08-03) classified it as **not
exploitable** and dropped it: the route is `POST` only and sits behind
`requireAuth`, so there is no attacker-craftable link, and the session cookie is
`SameSite=Lax`, so a cross-site form POST arrives without a session and is
bounced to `/login` before the tainted value is ever read. Reaching it needs
same-origin script execution or a same-site foothold, at which point the redirect
is not the interesting problem.

It is filed anyway because the check is *supposed* to be the boundary, its doc
comment claims it rejects anything that "could leave the site", and it does not.
A control that is weaker than its own docstring is worth correcting cheaply.

## Fix

Stop pattern-matching the string. Parse it and demand that it is a bare path:

- `url.Parse`, reject on error, then require `u.Scheme == ""`, `u.Host == ""` and
  `u.Opaque == ""` before accepting `u.EscapedPath()` (plus `u.RawQuery` when
  present).
- Belt and braces: reject any byte `< 0x20` or `== 0x7f` rather than only
  `\r`/`\n`.

Worth considering while in there: whether the client-supplied `return` field
should exist at all. `returnTarget` already reconstructs the canonical page URL
server-side, and the roster handler overrides it — the round-trip through the
client buys nothing except this attack surface. Dropping the field would delete
the control instead of fixing it, but it needs a look at whether any page's
correct return target is genuinely unknowable server-side.

## Acceptance

- `TestLanguageSwitchOnlyReturnsWithinTheApp` in `internal/web/i18n_test.go`
  gains a tab case (`"/\t/evil.com"`, and the percent-encoded `%09` form as it
  arrives over the wire) and any other control character worth pinning.
- The existing cases still pass — in particular
  `TestSwitcherReturnsToTheNormalisedRoster`, so the parse-based check does not
  mangle a legitimate `?sort=…&dir=…` return target.

## Comments

2026-08-03: Found by the pre-deployment security review. Note this is a bug in
code written the same week (`bb523cb`) — the original check was written as a
prefix test, which is exactly the shape that misses characters the browser
normalises away.
