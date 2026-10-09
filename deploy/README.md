# Managed deployment

Nginx/frpc/Caddy templates are generated from one model in internal/compiler.
Systemd and SSH/sudo fragments are installed from scripts/install-cloud.sh.
Windows SCM and ACL setup is in scripts/install-windows.ps1.

Do not hand-edit derived configuration. Cloud authorization registry is administrator-owned;
resource manifests cannot expand domain/path privileges or set arbitrary upstreams.
