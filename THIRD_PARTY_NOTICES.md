# Third-party components

Yandu does not fork Caddy, frp, Nginx or their protocols. Official source and release assets:

- Caddy 2.11.4 — Apache-2.0; https://github.com/caddyserver/caddy
- frp 0.71.0 — Apache-2.0; https://github.com/fatedier/frp
- Nginx — BSD-2-Clause; https://nginx.org/ (installed by the Ubuntu package manager, not bundled)
- React / React DOM / scheduler — MIT; https://github.com/facebook/react
- lucide-react — ISC; https://github.com/lucide-icons/lucide
- Go x/crypto, x/sys, x/term — BSD-3-Clause; https://go.googlesource.com/
- modernc.org/sqlite and related modernc modules — see included upstream licenses; https://gitlab.com/cznic/sqlite
- github.com/gofrs/flock — BSD-3-Clause; https://github.com/gofrs/flock

Full transitive Go inventory is recorded in go.mod/go.sum; UI inventory is in web/package-lock.json. The build collects publisher LICENSE/COPYING/NOTICE files into each package's licenses/ directory. No external fonts, tracking scripts or remote images are loaded by the local management UI.

Official release asset hashes are committed in release/dependencies.lock.json. Package SHA256SUMS provide integrity checks; these are not issuer identity signatures. This validation release is unsigned.
