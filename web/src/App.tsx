import {
  useCallback,
  useEffect,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import {
  ArrowDownToLine,
  ArrowRight,
  ArrowUpRight,
  Check,
  ChevronDown,
  ChevronRight,
  CircleHelp,
  Cloud,
  Copy,
  ExternalLink,
  FileImage,
  FileText,
  Folder,
  FolderOpen,
  HardDrive,
  History,
  KeyRound,
  Layers3,
  Link2,
  LoaderCircle,
  MoreHorizontal,
  Network,
  Pause,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Settings2,
  ShieldCheck,
  Unplug,
  X,
} from "lucide-react";

type Project = {
  id: string;
  displayName: string;
  siteId: string;
  projectBase: string;
  rootPath: string;
  resourcePrefixes: string[];
  publication: string;
  accessPolicy: string;
  cachePolicy: string;
  contentPolicy: string;
  revision: number;
  status: string;
  lastError?: string;
  username?: string;
  appliedRevision: number;
};
type Site = {
  id: string;
  domain: string;
  allowedPrefixes: string[];
  protectedPrefixes: string[];
};
type Operation = {
  id: string;
  projectId: string;
  action: string;
  stage: string;
  status: string;
  error?: string;
  startedAt: string;
  updatedAt: string;
};
type Route = {
  routeId: string;
  projectId: string;
  prefix: string;
  state: string;
};
type Status = {
  version: string;
  revision: number;
  projects: Project[];
  sites: Site[];
  roots: string[];
  operations: Operation[];
  reserved: Record<string, Route[]>;
  cloudRevisions: Record<string, number>;
  deviceId: string;
  sshFingerprint: string;
  components: {
    agent: boolean;
    fileService: boolean;
    tunnel: boolean;
    connection: boolean;
  };
};
type Entry = {
  name: string;
  directory: boolean;
  size: number;
  modified: string;
  url?: string;
  denied: boolean;
};
type Files = {
  entries: Entry[];
  total: number;
  offset: number;
  path: string;
  root: string;
};
type View = "overview" | "projects" | "files" | "connection" | "logs";
let csrf = "";
async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const r = await fetch("/api/v1" + path, {
    method,
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await r
    .json()
    .catch(() => ({ error: { message: "服务没有返回有效响应" } }));
  if (!r.ok) throw new Error(data.error?.message || "请求失败");
  return data as T;
}
const labels: Record<string, string> = {
  draft: "未发布",
  pending: "有待应用的修改",
  applying: "正在应用",
  active: "已生效",
  disabled: "已停用",
  disabled_pending: "已阻断 · 待同步",
  waiting_tunnel: "等待隧道",
  waiting_ingress: "等待入口同步",
  unverified: "外网待验证",
  local_invalid: "本地配置无效",
  recovering: "正在恢复",
 cloud_blocked: "云端已封禁",
};
const stageLabels: Record<string, string> = {
  queued: "等待执行",
  validate: "校验配置",
  local: "准备文件服务",
  tunnel: "注册隧道",
  cloud: "同步云端入口",
  probe: "验证外网资源",
  complete: "完成",
  failed: "失败",
};
const actionLabels: Record<string, string> = {
  apply: "应用项目",
  disable: "停用项目",
  remove: "删除映射",
  retry: "恢复与重试",
};
const empty: Project = {
  id: "",
  displayName: "",
  siteId: "",
  projectBase: "/blog",
  rootPath: "",
  resourcePrefixes: ["/blog/xxx/"],
  publication: "disabled",
  accessPolicy: "public_read",
  cachePolicy: "revalidate",
  contentPolicy: "media_only",
  revision: 0,
  status: "draft",
  appliedRevision: 0,
};
const viewTitles: Record<View, string> = {
  overview: "总览",
  projects: "项目管理",
  files: "目录与文件",
  connection: "服务器连接",
  logs: "操作记录",
};
function Pill({ p }: { p: Project }) {
  return (
    <span
      className={
        "pill " +
        (p.status === "active"
          ? "green"
          : (p.status.includes("invalid")||p.status==="cloud_blocked")
            ? "red"
            : p.status === "draft" || p.status === "disabled"
              ? "neutral"
              : "amber")
      }
    >
      <i />
      {labels[p.status] || p.status}
    </span>
  );
}
function Modal({
  title,
  description,
  children,
  onClose,
  wide = false,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
}) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [onClose]);
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <section
        className={"modal " + (wide ? "wide" : "")}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={(e) => e.stopPropagation()}
      >
        <header>
          <div>
            <h2>{title}</h2>
            {description && <p>{description}</p>}
          </div>
          <button className="icon-button" aria-label="关闭" onClick={onClose}>
            <X size={20} />
          </button>
        </header>
        {children}
      </section>
    </div>
  );
}
function Download(data: unknown, name: string) {
  const u = URL.createObjectURL(
    new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }),
  );
  const a = document.createElement("a");
  a.href = u;
  a.download = name;
  a.click();
  URL.revokeObjectURL(u);
}
export default function App() {
  const [view, setView] = useState<View>("overview");
  const [status, setStatus] = useState<Status>();
  const [auth, setAuth] = useState("loading");
  const [error, setError] = useState("");
  const [toast, setToast] = useState("");
  const [edit, setEdit] = useState<Project | null>(null);
  const [confirm, setConfirm] = useState<{
    p: Project;
    action: "apply" | "remove";
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState("");
  const notify = useCallback((text: string) => {
    setToast(text);
  }, []);
  useEffect(() => {
    if (toast) {
      const t = setTimeout(() => setToast(""), 4500);
      return () => clearTimeout(t);
    }
  }, [toast]);
  const refresh = useCallback(async () => {
    try {
      const s = await api<Status>("/status");
      setStatus(s);
      setError("");
    } catch (e) {
      setError((e as Error).message);
    }
  }, []);
  useEffect(() => {
    let live = true;
    async function boot() {
      try {
        const ticket = new URLSearchParams(location.hash.slice(1)).get(
          "ticket",
        );
        history.replaceState(null, "", location.pathname);
        const session = await api<{ csrf: string }>(
          "/session",
          ticket ? "POST" : "GET",
          ticket ? { ticket } : undefined,
        );
        csrf = session.csrf;
        if (live) {
          setAuth("ready");
          await refresh();
        }
      } catch (e) {
        if (live) {
          setAuth("locked");
          setError((e as Error).message);
        }
      }
    }
    void boot();
    window.addEventListener("hashchange",boot);
    return () => {
      window.removeEventListener("hashchange",boot);
      live = false;
    };
  }, [refresh]);
  useEffect(() => {
    if (auth !== "ready") return;
    const t = setInterval(() => void refresh(), 3000);
    return () => clearInterval(t);
  }, [auth, refresh]);
  async function action(p: Project, a: string, confirmed = false) {
    setBusy(true);
    try {
      await api("/projects/" + p.id + "/" + a, "POST", {
        expectedRevision: p.revision,
        confirmPublication: confirmed,
      });
      notify(
        a === "apply"
          ? "已开始应用，可在操作记录中查看进度"
          : "已在本地阻断，正在同步云端",
      );
      setConfirm(null);
      await refresh();
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      notify("已复制到剪贴板");
    } catch {
      notify("剪贴板不可用，请手动复制");
    }
  }
  function openNew() {
    setEdit({
      ...empty,
      siteId: status?.sites[0]?.id || "",
      rootPath: status?.roots[0] || "",
    });
  }
  if (auth === "loading")
    return (
      <div className="gate">
        <LoaderCircle className="spin" />
        <p>正在连接本机服务…</p>
      </div>
    );
  if (auth === "locked")
    return (
      <div className="gate">
        <Brand />
        <div className="gate-card">
          <ShieldCheck size={40} />
          <h1>从本机启动檐渡</h1>
          <p>{error}</p>
          <code>yandu ui</code>
          <small>通过桌面快捷方式或这条命令打开页面，取得本机会话。</small>
          <button onClick={() => location.reload()}>
            <RefreshCw size={16} />
            重新检查会话
          </button>
        </div>
      </div>
    );
  if (!status)
    return (
      <div className="gate">
        <LoaderCircle className="spin" />
        <p>{error || "正在读取项目…"}</p>
        <button onClick={() => void refresh()}>重试</button>
      </div>
    );
  const projects = status.projects.filter((p) =>
    (p.displayName + p.id + p.rootPath)
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const active = status.projects.filter((p) => p.status === "active").length;
  const pending = status.projects.filter(
    (p) => !["active", "draft", "disabled"].includes(p.status),
  ).length;
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Brand />
        <div className="workspace">
          <div className="workspace-icon">
            <HardDrive size={18} />
          </div>
          <div>
            <strong>本地工作空间</strong>
            <small>仅在这台设备上管理</small>
          </div>
          <ChevronDown size={14} />
        </div>
        <span className="nav-label">工作空间</span>
        <nav>
          {(
            [
              { id: "overview", name: "总览", icon: Layers3 },
              { id: "projects", name: "项目管理", icon: FolderOpen },
              { id: "files", name: "目录与文件", icon: FileImage },
              { id: "connection", name: "服务器连接", icon: Network },
              { id: "logs", name: "操作记录", icon: History },
            ] as const
          ).map((item) => (
            <button
              className={view === item.id ? "active" : ""}
              key={item.id}
              aria-label={item.name}
              onClick={() => setView(item.id)}
            >
              <item.icon size={18} />
              <span>{item.name}</span>
              {item.id === "projects" && (
                <span className="nav-count">{status.projects.length}</span>
              )}
              {item.id === "connection" && !status.components.connection && (
                <i className="nav-dot" />
              )}
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="local-badge">
            <ShieldCheck size={16} />
            <div>
              <strong>管理页仅限本机</strong>
              <small>127.0.0.1 · 本机会话已验证</small>
            </div>
          </div>
          <button
            onClick={async () => {
              try {
                Download(await api("/diagnostics"), "yandu-diagnostics.json");
              } catch (e) {
                notify((e as Error).message);
              }
            }}
          >
            <CircleHelp size={16} />
            导出诊断<span>↗</span>
          </button>
          <p>
            檐渡 Yandu <span>v{status.version}</span>
          </p>
        </div>
      </aside>
      <main>
        <div className="topbar">
          <div>
            <span>工作空间</span>
            <ChevronRight size={13} />
            <strong>{viewTitles[view]}</strong>
          </div>
          <div className="topbar-status">
            <i />
            {status.components.agent ? "本机服务运行中" : "本机服务离线"}
            <span className="avatar">本</span>
          </div>
        </div>
        <div className="content">
          {error && (
            <div className="error-banner">
              {error}
              <button onClick={() => void refresh()}>重试</button>
            </div>
          )}
          {view === "overview" && (
            <>
              <div className="page-heading">
                <div>
                  <div className="eyebrow">LOCAL FILES, CONNECTED.</div>
                  <h1>屋檐之下，也能抵达。</h1>
                  <p>为网站连接本地资源。文件留在这里，页面继续由云端提供。</p>
                </div>
                <button className="primary" onClick={openNew}>
                  <Plus size={17} />
                  新建项目
                </button>
              </div>
              <div className="connection-map">
                <div className="map-heading">
                  <span>
                    <Link2 size={16} />
                    你的资源连接
                  </span>
                  <span className="tiny-badge">同域名 · 按路径分流</span>
                </div>
                <div className="map-route">
                  <div className="map-node">
                    <div className="node-icon">
                      <HardDrive size={25} />
                    </div>
                    <div>
                      <strong>本地素材目录</strong>
                      <p>
                        {status.projects[0]?.rootPath ||
                          "选择并授权你的资源目录"}
                      </p>
                      <span>文件保留在设备上</span>
                    </div>
                  </div>
                  <div className="map-line">
                    <span>只读资源</span>
                    <div>
                      <i />
                      <i />
                      <i />
                      <ArrowRight size={17} />
                    </div>
                    <small>经过身份验证的 TLS 隧道</small>
                  </div>
                  <div className="map-node cloud-node">
                    <div className="node-icon">
                      <Cloud size={27} />
                    </div>
                    <div>
                      <strong>{status.sites[0]?.domain || "你的网站"}</strong>
                      <p>
                        {status.projects[0]?.resourcePrefixes[0] ||
                          "/blog/xxx/"}
                        <span className="filename">image.png</span>
                      </p>
                      <span>页面和业务 API 保留原有行为</span>
                    </div>
                  </div>
                </div>
                <div className="map-footer">
                  <ShieldCheck size={14} />
                  <span>仅显式登记的资源路径进入隧道</span>
                  <span className="map-footer-end">
                    目录不会复制到云端
                    <Check size={13} />
                  </span>
                </div>
              </div>
              <div className="stats">
                <Metric
                  label="管理项目"
                  value={status.projects.length}
                  note="一套工具，多个资源目录"
                  icon={<Folder size={19} />}
                />
                <Metric
                  label="已生效"
                  value={active}
                  note="已通过外网资源验证"
                  icon={<Link2 size={19} />}
                />
                <Metric
                  label="等待处理"
                  value={pending}
                  note={pending ? "查看状态或重试应用" : "当前没有待处理的应用"}
                  icon={<History size={19} />}
                />
              </div>
              <div className="section-heading">
                <div>
                  <h2>资源项目</h2>
                  <span>{status.projects.length} 个项目</span>
                </div>
                <button
                  className="text-button"
                  onClick={() => setView("projects")}
                >
                  管理全部项目
                  <ArrowRight size={15} />
                </button>
              </div>
              <ProjectTable
                projects={status.projects.slice(0, 5)}
                sites={status.sites}
                onEdit={setEdit}
                onNew={openNew}
                onApply={(p) => setConfirm({ p, action: "apply" })}
                onFiles={(p) => {
                  setSelected(p.id);
                  setView("files");
                }}
                onDisable={(p) => void action(p, "disable")}
              />
              {!status.components.connection && (
                <div className="onboarding">
                  <div className="onboarding-icon">
                    <Unplug size={23} />
                  </div>
                  <div>
                    <h3>先连接你的云端网站</h3>
                    <p>
                      导入云端安装器生成的加密连接包，再添加本地目录和资源路径。
                    </p>
                  </div>
                  <button onClick={() => setView("connection")}>
                    设置连接
                    <ArrowRight size={16} />
                  </button>
                </div>
              )}
              <div className="section-heading system-heading">
                <div>
                  <h2>服务状态</h2>
                </div>
                <small>页面每 3 秒更新</small>
              </div>
              <div className="services">
                {[
                  {
                    name: "本地管理",
                    ok: status.components.agent,
                    desc: "仅回环地址监听",
                  },
                  {
                    name: "文件服务",
                    ok: status.components.fileService,
                    desc: status.components.fileService
                      ? "Caddy 正在运行"
                      : "组件未启动，查看诊断",
                  },
                  {
                    name: "资源隧道",
                    ok: status.components.tunnel,
                    desc: status.components.tunnel
                      ? "客户端运行，注册结果见项目"
                      : "等待连接或组件安装",
                  },
                  {
                    name: "云端配置",
                    ok: status.components.connection,
                    desc: status.components.connection
                      ? "连接材料已导入"
                      : "尚未导入连接包",
                  },
                ].map((s) => (
                  <div key={s.name}>
                    <i className={s.ok ? "ok" : ""} />
                    <strong>{s.name}</strong>
                    <small>{s.desc}</small>
                  </div>
                ))}
              </div>
            </>
          )}
          {view === "projects" && (
            <>
              <Heading
                title="让每个目录，都有自己的路径。"
                subtitle="项目保存为草稿；明确应用后，才会向网站发布资源。"
                action={
                  <button className="primary" onClick={openNew}>
                    <Plus size={17} />
                    新建项目
                  </button>
                }
              />
              <div className="table-toolbar">
                <div className="search">
                  <Search size={17} />
                  <input
                    placeholder="搜索项目、名称或目录"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                </div>
                <span>{projects.length} 个项目</span>
                <button
                  onClick={async () => {
                    try {
                      Download(
                        await api("/config/export"),
                        "yandu-projects.json",
                      );
                    } catch (e) {
                      notify((e as Error).message);
                    }
                  }}
                >
                  <ArrowDownToLine size={16} />
                  导出草稿配置
                </button>
              </div>
              <ProjectTable
                projects={projects}
                sites={status.sites}
                onEdit={setEdit}
                onNew={openNew}
                onApply={(p) => setConfirm({ p, action: "apply" })}
                onFiles={(p) => {
                  setSelected(p.id);
                  setView("files");
                }}
                onDisable={(p) => void action(p, "disable")}
                onRemove={(p) => setConfirm({ p, action: "remove" })}
              />
              <div className="info-note">
                <ShieldCheck size={16} />
                <p>
                  停用或删除映射会保留拒绝规则。释放路径给原网站是独立操作，素材文件始终保留。
                </p>
              </div>
              <ReservedRoutes
                status={status}
                notify={notify}
                refresh={refresh}
              />
            </>
          )}
          {view === "files" && (
            <FilesView
              projects={status.projects}
              selected={selected}
              onSelected={setSelected}
              notify={notify}
              copy={copy}
            />
          )}
          {view === "connection" && (
            <Connection status={status} notify={notify} refresh={refresh} />
          )}
          {view === "logs" && (
            <>
              <Heading
                title="每一步应用，都有记录。"
                subtitle="查看校验、隧道注册、云端同步与资源验证的实际结果。"
                action={
                  <button onClick={() => void refresh()}>
                    <RefreshCw size={16} />
                    刷新
                  </button>
                }
              />
              <div className="logs-panel">
                {status.operations.length === 0 ? (
                  <div className="empty">
                    <History size={30} />
                    <h3>还没有操作记录</h3>
                    <p>保存项目并应用后，这里会记录各阶段结果。</p>
                  </div>
                ) : (
                  [...status.operations].reverse().map((op) => (
                    <article key={op.id} className="log-row">
                      <div className={"log-icon " + op.status}>
                        {op.status === "succeeded" ? (
                          <Check size={18} />
                        ) : op.status === "running" ? (
                          <LoaderCircle size={18} className="spin" />
                        ) : (
                          <History size={18} />
                        )}
                      </div>
                      <div className="log-body">
                        <strong>
                          {actionLabels[op.action] || op.action}
                          <span>{op.projectId}</span>
                        </strong>
                        <p>
                          {stageLabels[op.stage] || op.stage}
                          {op.error && " · " + op.error}
                        </p>
                        <code>{op.id}</code>
                      </div>
                      <div className="log-time">
                        <span>
                          {op.status === "succeeded"
                            ? "成功"
                            : op.status === "running"
                              ? "执行中"
                              : op.status === "interrupted"
                                ? "被重启中断"
                                : "待处理"}
                        </span>
                        <time>
                          {new Date(op.startedAt).toLocaleString("zh-CN")}
                        </time>
                      </div>
                    </article>
                  ))
                )}
              </div>
            </>
          )}
          <footer className="page-footer">
            <span>檐渡 · 让本地资源与网站相连</span>
            <span>
              <ShieldCheck size={13} />
              文件在本地，控制也在本地
            </span>
          </footer>
        </div>
      </main>
      {edit && (
        <ProjectForm
          p={edit}
          status={status}
          onClose={() => setEdit(null)}
          onSaved={async () => {
            setEdit(null);
            notify("项目已保存，应用后发布资源");
            await refresh();
          }}
        />
      )}
      {confirm && (
        <ConfirmProject
          p={confirm.p}
          action={confirm.action}
          sites={status.sites}
          busy={busy}
          onClose={() => setConfirm(null)}
          onConfirm={() => void action(confirm.p, confirm.action, true)}
        />
      )}
      {toast && (
        <div className="toast" role="status">
          <Check size={17} />
          {toast}
          <button aria-label="关闭提示" onClick={() => setToast("")}>
            <X size={15} />
          </button>
        </div>
      )}
    </div>
  );
}
function Brand() {
  return (
    <div className="brand">
      <svg width="35" height="35" viewBox="0 0 36 36" aria-hidden="true">
        <path
          d="M3 15 18 5l15 10M7 16h22M10 16v13M26 16v13M11 27c4-6 10-6 14 0"
          fill="none"
          stroke="currentColor"
          strokeWidth="2.7"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
      <strong>檐渡</strong>
      <span>YANDU</span>
    </div>
  );
}
function Heading({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle: string;
  action?: ReactNode;
}) {
  return (
    <div className="page-heading compact">
      <div>
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>
      {action}
    </div>
  );
}
function Metric({
  label,
  value,
  note,
  icon,
}: {
  label: string;
  value: number;
  note: string;
  icon: ReactNode;
}) {
  return (
    <div className="metric">
      <div>
        <span>{label}</span>
        {icon}
      </div>
      <strong>{value.toString().padStart(2, "0")}</strong>
      <p>{note}</p>
    </div>
  );
}
function ProjectTable({
  projects,
  sites,
  onEdit,
  onNew,
  onApply,
  onFiles,
  onDisable,
  onRemove,
}: {
  projects: Project[];
  sites: Site[];
  onEdit: (p: Project) => void;
  onNew: () => void;
  onApply: (p: Project) => void;
  onFiles: (p: Project) => void;
  onDisable: (p: Project) => void;
  onRemove?: (p: Project) => void;
}) {
  return (
    <div className="projects-table">
      {projects.length === 0 ? (
        <div className="empty">
          <div className="empty-icon">
            <FolderOpen size={28} />
          </div>
          <h3>从一个本地目录开始</h3>
          <p>登记资源范围，预览文件地址，确认后应用到网站。</p>
          <button onClick={onNew}>
            <Plus size={16} />
            添加第一个项目
          </button>
        </div>
      ) : (
        <table>
          <thead>
            <tr>
              <th>项目 / 本地目录</th>
              <th>网站资源路径</th>
              <th>状态</th>
              <th className="align-right">操作</th>
            </tr>
          </thead>
          <tbody>
            {projects.map((p) => (
              <tr key={p.id}>
                <td>
                  <div className="project-cell">
                    <div className="folder-icon">
                      <Folder size={19} />
                    </div>
                    <div>
                      <button
                        className="project-name"
                        onClick={() => onFiles(p)}
                      >
                        {p.displayName}
                      </button>
                      <code title={p.rootPath}>{p.rootPath}</code>
                    </div>
                  </div>
                </td>
                <td>
                  <span className="domain-text">
                    {sites.find((s) => s.id === p.siteId)?.domain || p.siteId}
                  </span>
                  {p.resourcePrefixes.map((pre) => (
                    <code className="route-code" key={pre}>
                      {pre}
                      <span>*</span>
                    </code>
                  ))}
                </td>
                <td>
                  <Pill p={p} />
                  {p.accessPolicy === "basic_auth" && (
                    <small className="access-label">
                      <KeyRound size={11} />
                      密码保护
                    </small>
                  )}
                  {p.lastError && (
                    <small className="row-error" title={p.lastError}>
                      {p.lastError}
                    </small>
                  )}
                </td>
                <td>
                  <div className="row-actions">
                    <button
                      className="icon-button"
                      title="编辑项目"
                      aria-label={"编辑 " + p.displayName}
                      onClick={() => onEdit(p)}
                    >
                      <Pencil size={15} />
                    </button>
                    {p.publication === "active" ? (
                      <button
                        className="icon-button"
                        title="停用项目"
                        aria-label={"停用 " + p.displayName}
                        onClick={() => onDisable(p)}
                      >
                        <Pause size={16} />
                      </button>
                    ) : null}
                    <button className="apply-button" onClick={() => onApply(p)}>
                      {p.status === "active" ? "重新验证" : "应用"}
                      <ArrowUpRight size={14} />
                    </button>
                    {onRemove && (
                      <button
                        className="icon-button"
                        title="删除映射"
                        aria-label={"删除 " + p.displayName}
                        onClick={() => onRemove(p)}
                      >
                        <X size={15} />
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
function ProjectForm({
  p,
  status,
  onClose,
  onSaved,
}: {
  p: Project;
  status: Status;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [form, setForm] = useState(p);
  const [prefix, setPrefix] = useState(p.resourcePrefixes.join("\n"));
  const [password, setPassword] = useState("");
  const [grant, setGrant] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const set = (key: keyof Project, value: string) =>
    setForm((f) => ({ ...f, [key]: value }));
  const site = status.sites.find((s) => s.id === form.siteId);
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (grant) await api("/roots", "POST", { path: form.rootPath });
      await api(
        p.revision ? "/projects/" + p.id : "/projects",
        p.revision ? "PUT" : "POST",
        {
          project: {
            ...form,
            resourcePrefixes: prefix
              .split(/\n|,/)
              .map((p) => p.trim())
              .filter(Boolean),
          },
          password,
          expectedRevision: p.revision,
        },
      );
      onSaved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      wide
      title={p.revision ? "编辑资源项目" : "新建资源项目"}
      description="指定本地目录与网站资源范围，保存后仍需确认应用。"
      onClose={onClose}
    >
      <form onSubmit={save}>
        <div className="form-body">
          {!status.sites.length && (
            <div className="warning">
              请先到服务器连接页导入连接包，获取站点及路径授权。
            </div>
          )}
          {error && <div className="warning">{error}</div>}
          <div className="form-grid">
            <label>
              项目名称
              <input
                autoFocus
                required
                value={form.displayName}
                placeholder="个人博客"
                onChange={(e) => set("displayName", e.target.value)}
              />
            </label>
            <label>
              项目标识
              <input
                required
                disabled={!!p.revision}
                pattern={"[a-z][a-z0-9\\-]*"}
                maxLength={48}
                value={form.id}
                placeholder="blog"
                onChange={(e) => set("id", e.target.value)}
              />
              <small>小写字母、数字或连字符</small>
            </label>
          </div>
          <label>
            本地素材目录
            <div className="input-icon">
              <Folder size={17} />
              <input
                required
                value={form.rootPath}
                placeholder="D:/oss/blog"
                onChange={(e) => set("rootPath", e.target.value)}
              />
            </div>
            <small>
              服务账号必须具有读取权限；不支持符号链接或网络映射盘。
            </small>
          </label>
          {!status.roots.some(
            (root) =>
              form.rootPath === root ||
              form.rootPath.startsWith(root + "/") ||
              form.rootPath.startsWith(root + "\\"),
          ) && (
            <label className="check-label">
              <input
                type="checkbox"
                checked={grant}
                onChange={(e) => setGrant(e.target.checked)}
              />
              我授权檐渡管理并读取这个素材目录
            </label>
          )}
          <div className="form-grid">
            <label>
              网站站点
              <select
                required
                value={form.siteId}
                onChange={(e) => set("siteId", e.target.value)}
              >
                <option value="">选择已授权站点</option>
                {status.sites.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.domain} · {s.id}
                  </option>
                ))}
              </select>
            </label>
            <label>
              页面基础路径
              <input
                required
                value={form.projectBase}
                placeholder="/blog"
                onChange={(e) => set("projectBase", e.target.value)}
              />
              <small>仅用于文件路径转换，页面继续走云端</small>
            </label>
          </div>
          <label>
            资源路径范围
            <textarea
              required
              rows={3}
              value={prefix}
              onChange={(e) => setPrefix(e.target.value)}
              placeholder="/blog/xxx/"
            />
            <small>每行一个，以 / 结尾。仅这些范围会连接到本地。</small>
          </label>
          <div className="mapping-preview">
            <span>路径预览</span>
            <code>
              https://{site?.domain || "example.com"}
              {prefix.split("\n")[0]}image.png
            </code>
            <ArrowDownToLine size={14} />
            <code>
              {form.rootPath || "D:/oss/blog"}/
              {prefix.split("\n")[0].replace(form.projectBase + "/", "")}
              image.png
            </code>
          </div>
          <div className="form-grid">
            <label>
              访问权限
              <select
                value={form.accessPolicy}
                onChange={(e) => set("accessPolicy", e.target.value)}
              >
                <option value="public_read">公开只读</option>
                <option value="basic_auth">独立密码保护</option>
              </select>
            </label>
            <label>
              缓存策略
              <select
                value={form.cachePolicy}
                onChange={(e) => set("cachePolicy", e.target.value)}
              >
                <option value="revalidate">每次重验证（默认）</option>
                <option value="immutable">内容哈希文件，长期缓存</option>
              </select>
            </label>
          </div>
          {form.accessPolicy === "basic_auth" && (
            <div className="form-grid">
              <label>
                资源用户名
                <input
                  required
                  value={form.username || ""}
                  onChange={(e) => set("username", e.target.value)}
                  autoComplete="off"
                />
              </label>
              <label>
                资源密码
                <input
                  type="password"
                  required={!p.revision || p.accessPolicy !== "basic_auth"}
                  minLength={12}
                  maxLength={72}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="new-password"
                  placeholder={p.revision ? "留空保留原密码" : "至少 12 个字符"}
                />
              </label>
            </div>
          )}
          <div className="info-note">
            <ShieldCheck size={16} />
            <p>
              仅发布图片、音频与视频。HTML、JavaScript、SVG 和目录列表会被阻断。
            </p>
          </div>
        </div>
        <div className="modal-actions">
          <button type="button" onClick={onClose}>
            取消
          </button>
          <button
            className="primary"
            disabled={busy || !status.sites.length}
            type="submit"
          >
            {busy ? (
              <LoaderCircle size={16} className="spin" />
            ) : (
              <Check size={16} />
            )}
            保存项目
          </button>
        </div>
      </form>
    </Modal>
  );
}
function ConfirmProject({
  p,
  action,
  sites,
  busy,
  onClose,
  onConfirm,
}: {
  p: Project;
  action: string;
  sites: Site[];
  busy: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const [accepted, setAccepted] = useState(false);
  return (
    <Modal
      title={action === "apply" ? "确认发布这些资源" : "删除项目映射"}
      description={
        action === "apply"
          ? "应用将更新本地服务、资源隧道和云端入口。"
          : "本地文件会保留，资源路径继续保持拒绝状态。"
      }
      onClose={onClose}
    >
      <div className="form-body">
        <div className="confirm-project">
          <Folder size={22} />
          <div>
            <strong>{p.displayName}</strong>
            <code>{p.rootPath}</code>
          </div>
        </div>
        {p.resourcePrefixes.map((pre) => (
          <code className="confirm-path" key={pre}>
            {sites.find((s) => s.id === p.siteId)?.domain}
            {pre}*
          </code>
        ))}
        {action === "apply" && (
          <>
            <div className="warning">
              {p.accessPolicy === "public_read"
                ? "这些资源将可由互联网上的任何访问者读取。管理页仍然只在本机开放。"
                : "访问者需要独立资源密码。应用后需输入密码完成外网验证。"}
            </div>
            <label className="check-label">
              <input
                type="checkbox"
                checked={accepted}
                onChange={(e) => setAccepted(e.target.checked)}
              />
              我确认该目录符合发布规则，并允许按所选权限发布
            </label>
          </>
        )}
      </div>
      <div className="modal-actions">
        <button onClick={onClose}>取消</button>
        <button
          className={action === "apply" ? "primary" : "danger"}
          disabled={busy || (action === "apply" && !accepted)}
          onClick={onConfirm}
        >
          {busy ? (
            <LoaderCircle className="spin" size={16} />
          ) : (
            <ArrowUpRight size={16} />
          )}{" "}
          {action === "apply" ? "确认并应用" : "删除映射"}
        </button>
      </div>
    </Modal>
  );
}
function FilesView({
  projects,
  selected,
  onSelected,
  notify,
  copy,
}: {
  projects: Project[];
  selected: string;
  onSelected: (s: string) => void;
  notify: (s: string) => void;
  copy: (s: string) => void;
}) {
  const [path, setPath] = useState("");
  const [offset, setOffset] = useState(0);
  const [files, setFiles] = useState<Files>();
  const [error, setError] = useState("");
  const [folder, setFolder] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const id = selected || projects[0]?.id;
  const p = projects.find((p) => p.id === id);
  const load = useCallback(async () => {
    if (!id) return;
    try {
      setFiles(
        await api<Files>(
          "/files?projectId=" +
            encodeURIComponent(id) +
            "&path=" +
            encodeURIComponent(path) +
            "&offset=" +
            offset,
        ),
      );
      setError("");
    } catch (e) {
      setError((e as Error).message);
    }
  }, [id, path, offset]);
  useEffect(() => {
    setFiles(undefined);
    void load();
  }, [load]);
  async function create(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await api("/directories", "POST", {
        projectId: id,
        path: (path ? path + "/" : "") + folder,
      });
      setFolder(null);
      notify("子目录已创建");
      await load();
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <Heading
        title="文件在本地，地址在这里。"
        subtitle="浏览已授权的项目目录，复制资源地址。未命中资源范围的文件不会发布。"
      />
      <div className="file-toolbar">
        <div className="select-project">
          <Folder size={17} />
          <select
            value={id || ""}
            onChange={(e) => {
              onSelected(e.target.value);
              setPath("");
              setOffset(0);
            }}
          >
            {projects.length === 0 && <option>尚无项目</option>}
            {projects.map((p) => (
              <option value={p.id} key={p.id}>
                {p.displayName}
              </option>
            ))}
          </select>
        </div>
        <button onClick={() => void load()} disabled={!id}>
          <RefreshCw size={16} />
          刷新
        </button>
        <button onClick={() => setFolder("")} disabled={!id}>
          <Plus size={16} />
          创建子目录
        </button>
      </div>
      {p && (
        <div className="breadcrumbs">
          <button title={p.rootPath}
            onClick={() => {
              setPath("");
              setOffset(0);
            }}
          >
            <HardDrive size={15} />
            {p.rootPath}
          </button>
          {path
            .split("/")
            .filter(Boolean)
            .map((s, i) => (
              <span key={i}>
                <ChevronRight size={13} />
                <button
                  onClick={() => {
                    setPath(
                      path
                        .split("/")
                        .slice(0, i + 1)
                        .join("/"),
                    );
                    setOffset(0);
                  }}
                >
                  {s}
                </button>
              </span>
            ))}
        </div>
      )}
      {p&&p.status!=="active"&&<div className="info-note file-preview-note"><ShieldCheck size={16}/><p>项目尚未生效，资源地址仅供配置预览。应用并通过外网验证后才能读取。</p></div>}
      {error && <div className="warning">{error}</div>}
      <div className="projects-table">
        {!id ? (
          <div className="empty">
            <FolderOpen size={30} />
            <h3>先添加一个资源项目</h3>
            <p>项目中的授权目录会显示在这里。</p>
          </div>
        ) : !files && !error ? (
          <div className="empty">
            <LoaderCircle className="spin" />
          </div>
        ) : files?.entries.length === 0 ? (
          <div className="empty">
            <Folder size={28} />
            <h3>这个目录还是空的</h3>
            <p>可创建子目录；将素材文件放入对应资源范围后即可读取。</p>
          </div>
        ) : (
          <table className="file-table">
            <thead>
              <tr>
                <th>文件名</th>
                <th>大小</th>
                <th>修改时间</th>
                <th>资源地址</th>
              </tr>
            </thead>
            <tbody>
              {files?.entries.map((f) => (
                <tr key={f.name}>
                  <td>
                    <button
                      className="file-name"
                      disabled={!f.directory || f.denied}
                      onClick={() => {
                        setPath((path ? path + "/" : "") + f.name);
                        setOffset(0);
                      }}
                    >
                      {f.directory ? (
                        <Folder size={19} />
                      ) : (
                        <FileImage size={18} />
                      )}{" "}
                      {f.name}
                      {f.denied && <span>链接已阻断</span>}
                    </button>
                  </td>
                  <td>{f.directory ? "—" : size(f.size)}</td>
                  <td>{new Date(f.modified).toLocaleDateString("zh-CN")}</td>
                  <td>
                    {f.url ? (
                      <div className="file-address"><code title={f.url}>{f.url}</code><button
                        className="text-button"
                        onClick={() => copy(f.url!)}
                      >
                        <Copy size={14} />
                        复制地址
                      </button></div>
                    ) : (
                      <span className="muted">
                        {f.directory ? "目录不公开" : "未在资源范围内"}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {files && files.total > 100 && (
        <div className="pagination">
          <span>共 {files.total} 项</span>
          <button
            disabled={offset === 0}
            onClick={() => setOffset(offset - 100)}
          >
            上一页
          </button>
          <span>{Math.floor(offset / 100) + 1}</span>
          <button
            disabled={offset + 100 >= files.total}
            onClick={() => setOffset(offset + 100)}
          >
            下一页
          </button>
        </div>
      )}
      <div className="info-note">
        <ShieldCheck size={16} />
        <p>目录列表只在本机显示。删除项目映射不会删除文件。</p>
      </div>
      {folder !== null && (
        <Modal title="创建子目录" onClose={() => setFolder(null)}>
          <form onSubmit={create}>
            <div className="form-body">
              <label>
                目录名称
                <input
                  required
                  autoFocus
                  value={folder}
                  
                  onChange={(e) => setFolder(e.target.value)}
                />
              </label>
              <small>
                将在 {p?.rootPath}/{path} 下创建。
              </small>
            </div>
            <div className="modal-actions">
              <button type="button" onClick={() => setFolder(null)}>
                取消
              </button>
              <button className="primary" disabled={busy}>
                创建目录
              </button>
            </div>
          </form>
        </Modal>
      )}
    </>
  );
}
function size(n: number) {
  if (n < 1024) return n + " B";
  if (n < 1048576) return (n / 1024).toFixed(1) + " KB";
  return (n / 1048576).toFixed(1) + " MB";
}
function Connection({
  status,
  notify,
  refresh,
}: {
  status: Status;
  notify: (s: string) => void;
  refresh: () => Promise<void>;
}) {
  const [bundle, setBundle] = useState<File | null>(null);
  const [pass, setPass] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState("");
  const [verifyProject, setVerifyProject] = useState<Project | null>(null);
  const [resourcePass, setResourcePass] = useState("");
  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!bundle) return;
    setBusy(true);
    try {
      await api("/connection/import", "POST", {
        bundle: JSON.parse(await bundle.text()),
        passphrase: pass,
      });
      setPass("");
      setResult("连接包已导入。项目保持停用，请逐一确认应用。");
      await refresh();
    } catch (e) {
      setResult((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <Heading
        title="连接一次，管理多个项目。"
        subtitle="使用加密连接包配对你的云服务器。域名与资源路径权限由云端管理员授予。"
      />
      <div className="connection-columns">
        <section className="panel">
          <div className="panel-title">
            <KeyRound size={20} />
            <h2>导入连接包</h2>
          </div>
          <p className="panel-description">
            在云端完成安装后，取得 .yandu-profile 文件和单独交付的口令。
          </p>
          <form onSubmit={submit}>
            <label className="upload-zone">
              <ArrowDownToLine size={25} />
              <strong>{bundle?.name || "选择加密连接包"}</strong>
              <span>只接受 .yandu-profile 文件 · 最大 256 KB</span>
              <input
                type="file"
                accept=".yandu-profile"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f && f.size > 262144) {
                    notify("连接包超过 256 KB");
                    return;
                  }
                  setBundle(f || null);
                }}
              />
            </label>
            <label>
              连接包口令
              <input
                type="password"
                value={pass}
                onChange={(e) => setPass(e.target.value)}
                autoComplete="off"
                required
                placeholder="输入单独交付的口令"
              />
            </label>
            <button className="primary full" disabled={!bundle || busy}>
              {busy ? (
                <LoaderCircle size={16} className="spin" />
              ) : (
                <Link2 size={16} />
              )}
              导入并配对
            </button>
          </form>
          {result && (
            <p className="connection-result" role="status">
              {result}
            </p>
          )}
        </section>
        <section className="panel">
          <div className="panel-title">
            <ShieldCheck size={20} />
            <h2>连接与信任</h2>
          </div>
          {status.components.connection ? (
            <>
              <dl className="connection-details">
                <dt>设备身份</dt>
                <dd>{status.deviceId}</dd>
                <dt>SSH 主机指纹</dt>
                <dd>
                  <code>{status.sshFingerprint}</code>
                </dd>
                <dt>授权站点</dt>
                <dd>
                  {status.sites.map((s) => (
                    <div key={s.id}>
                      <strong>{s.domain}</strong>
                      <small>{s.allowedPrefixes.join(" · ")}</small>
                    </div>
                  ))}
                </dd>
              </dl>
              <button
                disabled={busy}
                onClick={async () => {
                  setBusy(true);
                  try {
                    await api("/connection/test", "POST", {});
                    notify("SSH 指纹与设备授权验证通过");
                  } catch (e) {
                    notify((e as Error).message);
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                <RefreshCw size={16} />
                测试配置通道
              </button>
            </>
          ) : (
            <div className="connection-empty">
              <Cloud size={37} />
              <h3>尚未连接服务器</h3>
              <p>导入连接包后，会在这里显示服务器指纹、设备身份和授权范围。</p>
            </div>
          )}
          <div className="info-note">
            <ShieldCheck size={16} />
            <p>主机指纹变化会阻断连接。连接包不含 root、DNS 或网站证书私钥。</p>
          </div>
        </section>
      </div>
      {status.projects.some(
        (p) => p.accessPolicy === "basic_auth" && p.publication === "active",
      ) && (
        <section className="panel protected-panel">
          <h2>验证密码资源</h2>
          <p>使用独立资源密码检查实际外网响应；密码不会保存或写入日志。</p>
          {status.projects
            .filter(
              (p) =>
                p.accessPolicy === "basic_auth" && p.publication === "active",
            )
            .map((p) => (
              <button key={p.id} onClick={() => setVerifyProject(p)}>
                <KeyRound size={15} />
                {p.displayName}
                <ExternalLink size={14} />
              </button>
            ))}
        </section>
      )}
      <section className="panel install-guide">
        <div className="panel-title">
          <Settings2 size={20} />
          <h2>还没有连接包？</h2>
        </div>
        <p>
          从项目的 Linux 发布包运行云端安装器。已有网站需明确选择 Nginx
          的站点配置文件和资源授权范围。
        </p>
        <pre>
          sudo bash ./install-cloud.sh \ --mode existing-nginx --site-id main \
          --domain example.com \ --server-config
          /etc/nginx/sites-available/main.conf \ --allow /blog/
        </pre>
        <small>完整部署步骤见发布包中的 README 和 docs/DEPLOYMENT.md。</small>
      </section>
      {verifyProject && (
        <Modal
          title={"验证 " + verifyProject.displayName}
          onClose={() => setVerifyProject(null)}
        >
          <form
            onSubmit={async (e) => {
              e.preventDefault();
              setBusy(true);
              try {
                await api("/projects/" + verifyProject.id + "/verify", "POST", {
                  password: resourcePass,
                });
                setResourcePass("");
                setVerifyProject(null);
                notify("密码资源已通过外网验证");
                await refresh();
              } catch (e) {
                notify((e as Error).message);
              } finally {
                setBusy(false);
              }
            }}
          >
            <div className="form-body">
              <label>
                资源密码
                <input
                  required
                  autoFocus
                  type="password"
                  autoComplete="off"
                  value={resourcePass}
                  onChange={(e) => setResourcePass(e.target.value)}
                />
              </label>
            </div>
            <div className="modal-actions">
              <button className="primary" disabled={busy}>
                验证资源
              </button>
            </div>
          </form>
        </Modal>
      )}
    </>
  );
}
function ReservedRoutes({
  status,
  notify,
  refresh,
}: {
  status: Status;
  notify: (s: string) => void;
  refresh: () => Promise<void>;
}) {
  const [release, setRelease] = useState<{ site: string; route: Route } | null>(
    null,
  );
  const [busy, setBusy] = useState(false);
  const rows = Object.entries(status.reserved).flatMap(([site, routes]) =>
    routes.map((route) => ({ site, route })),
  );
  if (!rows.length) return null;
  return (
    <section className="panel reserved-panel">
      <h2>已保留的旧路径</h2>
      <p>旧资源路径保持 404。明确释放后，才交回原网站处理。</p>
      {rows.map((item) => (
        <div className="reserved-row" key={item.route.routeId}>
          <code>{item.route.prefix}</code>
          <span>{item.route.projectId}</span>
          <button onClick={() => setRelease(item)}>
            释放给网站
            <ArrowUpRight size={13} />
          </button>
        </div>
      ))}
      {release && (
        <Modal
          title="释放资源路径给网站"
          description="该路径将不再由檐渡拒绝，原网站可能返回页面或其他内容。"
          onClose={() => setRelease(null)}
        >
          <div className="form-body">
            <code>{release.route.prefix}</code>
          </div>
          <div className="modal-actions">
            <button onClick={() => setRelease(null)}>取消</button>
            <button
              className="danger"
              disabled={busy}
              onClick={async () => {
                setBusy(true);
                try {
                  await api("/routes/release", "POST", {
                    siteId: release.site,
                    routeId: release.route.routeId,
                    confirm: true,
                  });
                  setRelease(null);
                  notify("路径已释放给网站");
                  await refresh();
                } catch (e) {
                  notify((e as Error).message);
                } finally {
                  setBusy(false);
                }
              }}
            >
              确认释放
            </button>
          </div>
        </Modal>
      )}
    </section>
  );
}
