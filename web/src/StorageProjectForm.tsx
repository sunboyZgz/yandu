import { useState, type FormEvent } from "react";
import { Check, LoaderCircle } from "lucide-react";
import DirectoryField from "./DirectoryField";
import {
  Field,
  Required,
  directoryError,
  feedback,
  focusFirstError,
  projectIdError,
  projectNameError,
} from "./FormField";

export default function StorageProjectForm({
  projects,
  csrf,
  onCreated,
  refresh,
}: {
  projects: { id: string }[];
  csrf: string;
  onCreated: (id: string) => void;
  refresh: () => Promise<void>;
}) {
  const [id, setId] = useState("");
  const [name, setName] = useState("");
  const [root, setRoot] = useState("");
  const [quota, setQuota] = useState("10");
  const [grant, setGrant] = useState(false);
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  function clear(key: string) {
    setErrors((old) => ({ ...old, [key]: "" }));
    setError("");
    setNotice("");
  }
  async function request<T>(url: string, body: unknown): Promise<T> {
    const response = await fetch("/api/v1" + url, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify(body),
    });
    const data = await response.json();
    if (!response.ok)
      throw new Error(data.error?.message ?? "请求失败，请重试");
    return data;
  }
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const quotaBytes = Number(quota) * 1024 ** 3;
    const issues: Record<string, string> = {
      id: projectIdError(id),
      name: projectNameError(name),
      root: directoryError(root),
      quota:
        !Number.isSafeInteger(quotaBytes) || quotaBytes <= 0
          ? "请填写大于 0 的有效配额，可填写小数"
          : "",
      grant: !grant ? "请确认目录授权后再创建项目" : "",
    };
    if (projects.some((project) => project.id === id))
      issues.id = "项目标识已存在，请换一个标识";
    setErrors(issues);
    setError("");
    setNotice("");
    if (Object.values(issues).some(Boolean)) {
      focusFirstError(event.currentTarget);
      return;
    }
    setBusy(true);
    try {
      await request("/roots", { path: root.trim() });
      const project = await request<{ id: string; revision: number }>(
        "/projects",
        {
          project: {
            id,
            displayName: name.trim(),
            mode: "application_storage",
            rootPath: root.trim(),
            quotaBytes,
            siteId: "",
            projectBase: "",
            resourcePrefixes: [],
            publication: "disabled",
            accessPolicy: "",
            contentPolicy: "",
            cachePolicy: "",
          },
          expectedRevision: 0,
        },
      );
      // Once saved, always select the project, even if enabling fails. Retrying
      // creation would otherwise report a duplicate ID and hide the saved draft.
      onCreated(project.id);
      setId("");
      setName("");
      setRoot("");
      setGrant(false);
      setErrors({});
      try {
        await request(`/projects/${project.id}/apply`, {
          expectedRevision: project.revision,
          confirmPublication: true,
        });
        setNotice(`项目“${name.trim()}”已创建，正在启用。可在下方查看状态。`);
      } catch (e) {
        setError(
          `项目已创建，启用未完成：${(e as Error).message}。请在下方选择该项目并点击“启用项目”重试。`,
        );
      }
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  const props = (key: string) => ({
    id: `storage-${key}`,
    ...feedback(`storage-${key}`, errors[key]),
  });
  return (
    <form onSubmit={create} className="storage-form" noValidate>
      <p className="form-help">
        标有“必填”的项目需要填写或选择。存储项目将管理此目录，请选择已创建的空文件夹。
      </p>
      {error && (
        <div className="warning" role="alert">
          {error}
        </div>
      )}
      {notice && (
        <p className="success-note" role="status">
          {notice}
        </p>
      )}
      <fieldset className="form-fields" disabled={busy}>
        <div className="form-grid">
          <Field
            id="storage-id"
            label="项目 ID"
            required
            hint="以小写字母开头，支持小写字母、数字和连字符，最多 48 位。"
            error={errors.id}
          >
            <input
              {...props("id")}
              required
              maxLength={48}
              value={id}
              placeholder="my-app"
              onChange={(event) => {
                setId(event.target.value);
                clear("id");
              }}
            />
          </Field>
          <Field
            id="storage-name"
            label="项目名称"
            required
            hint="用于在管理页面识别此项目。"
            error={errors.name}
          >
            <input
              {...props("name")}
              required
              value={name}
              placeholder="应用文件"
              onChange={(event) => {
                setName(event.target.value);
                clear("name");
              }}
            />
          </Field>
        </div>
        <DirectoryField
          id="storage-root"
          label="存储目录"
          value={root}
          csrf={csrf}
          error={errors.root}
          disabled={busy}
          onChange={(path) => {
            setRoot(path);
            setGrant(false);
            clear("root");
            clear("grant");
          }}
          hint="请选择已有的空文件夹，目录不能与其他项目相互包含。"
        />
        <Field
          id="storage-quota"
          label="配额（GiB）"
          required
          hint="1 GiB = 1024 MiB；可填写小数，例如 0.5。"
          error={errors.quota}
        >
          <input
            {...props("quota")}
            type="number"
            min="0"
            step="any"
            value={quota}
            required
            onChange={(event) => {
              setQuota(event.target.value);
              clear("quota");
            }}
          />
        </Field>
        <div className="form-field">
          <label className="check-label">
            <input
              {...props("grant")}
              type="checkbox"
              required
              checked={grant}
              onChange={(event) => {
                setGrant(event.target.checked);
                clear("grant");
              }}
            />
            <span>
              我授权檐渡管理此目录中的存储文件
              <Required />
            </span>
          </label>
          <small id="storage-grant-hint">
            选择目录后需勾选确认，再创建并启用项目。
          </small>
          {errors.grant && (
            <span id="storage-grant-error" className="field-error">
              {errors.grant}
            </span>
          )}
        </div>
      </fieldset>
      <button className="primary" type="submit" disabled={busy}>
        {busy ? (
          <LoaderCircle size={16} className="spin" />
        ) : (
          <Check size={16} />
        )}
        {busy ? "正在创建并启用…" : "授权目录并启用项目"}
      </button>
    </form>
  );
}
