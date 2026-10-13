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

v0.2 storage also uses upstream SFTPGo Community 2.7.6 (AGPL-3.0), tusd 2.10.1 and tus-js-client 4.3.1 (MIT), goose 3.26.0 (MIT), oapi-codegen 2.5.1 (Apache-2.0), OpenAPI Generator 7.17.0 (Apache-2.0), Node 22.23.1 (MIT and bundled notices), Sharp 0.35.5 (Apache-2.0), libvips (LGPL-2.1-or-later), libheif 1.23.6 (LGPL-3.0-or-later), libde265 1.0.16 (LGPL-3.0-or-later), libpng 1.6.58 (libpng-2.0), FFmpeg 9.0.2 configured with GPL x264 (GPL-2.0-or-later), x264 r3108 (GPL-2.0-only), ExifTool (Artistic or GPL) and exiftool-vendored 39.0.1 (MIT). SFTPGo runs as a separate upstream process through its public REST API; it has its own database. Yandu does not rewrite these transfer or codec implementations.

Native bundles collect upstream licenses, source inputs and build inventories. Distributors must retain the corresponding-source and notices required by each upstream component. The macOS codec builder records a macOS 14 deployment target and seals dynamic libraries into the bundle. Media npm native payloads are locked by npm integrity hashes. Component receipts are integrity records, not publisher signatures.
