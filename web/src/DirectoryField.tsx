import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ArrowUp, Folder, FolderOpen, LoaderCircle } from "lucide-react";
import Modal from "./Modal";
import { Field, feedback } from "./FormField";

type Listing = {
  path: string;
  parent: string;
  directories: { name: string; path: string }[];
  shortcuts: { name: string; path: string }[];
  selectable: boolean;
};
export default function DirectoryField({
  id,
  label,
  value,
  onChange,
  csrf,
  error,
  disabled,
  hint,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (path: string) => void;
  csrf: string;
  error?: string;
  disabled?: boolean;
  hint?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Field
        id={id}
        label={label}
        required
        error={error}
        hint={
          hint ?? "可选择文件夹，也可粘贴绝对路径。选择后仍需确认目录授权。"
        }
      >
        <div className="directory-input">
          <input
            id={id}
            name="root"
            value={value}
            onChange={(event) => onChange(event.target.value)}
            required
            disabled={disabled}
            placeholder="选择文件夹或输入绝对路径"
            {...feedback(id, error)}
          />
          <button
            type="button"
            disabled={disabled}
            onClick={() => setOpen(true)}
          >
            <FolderOpen size={16} />
            选择文件夹
          </button>
        </div>
      </Field>
      {open &&
        createPortal(
          <DirectoryPicker
            csrf={csrf}
            initial={value}
            onClose={() => setOpen(false)}
            onSelect={(path) => {
              onChange(path);
              setOpen(false);
            }}
          />,
          document.body,
        )}
    </>
  );
}

function DirectoryPicker({
  initial,
  csrf,
  onSelect,
  onClose,
}: {
  initial: string;
  csrf: string;
  onSelect: (path: string) => void;
  onClose: () => void;
}) {
  const [listing, setListing] = useState<Listing>();
  const [path, setPath] = useState(initial);
  const [loading, setLoading] = useState(true);
  const [nativeBusy, setNativeBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const sequence = useRef(0);
  const nativeController = useRef<AbortController | null>(null);
  async function browse(next: string, signal?: AbortSignal) {
    const current = ++sequence.current;
    setLoading(true);
    setError("");
    try {
      const res = await fetch(
        `/api/v1/directories/browse?path=${encodeURIComponent(next)}`,
        { signal },
      );
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message ?? "目录读取失败");
      if (current === sequence.current) {
        setListing(data);
        setPath(data.path);
      }
    } catch (e) {
      if (current === sequence.current && !signal?.aborted)
        setError((e as Error).message);
    } finally {
      if (current === sequence.current) setLoading(false);
    }
  }
  useEffect(() => {
    const controller = new AbortController();
    void browse(initial, controller.signal);
    return () => {
      ++sequence.current;
      controller.abort();
      nativeController.current?.abort();
    };
  }, [initial]);
  async function pickNative() {
    if (nativeBusy) return;
    const controller = new AbortController();
    nativeController.current = controller;
    setNativeBusy(true);
    setNotice("");
    try {
      const res = await fetch("/api/v1/directories/pick", {
        method: "POST",
        headers: { "X-CSRF-Token": csrf },
        signal: controller.signal,
      });
      const data = await res.json();
      if (!res.ok)
        throw new Error(
          data.error?.message ?? "系统选择器不可用，请使用下方的目录浏览器",
        );
      if (!data.cancelled && data.path) onSelect(data.path);
      else setNotice("已取消系统选择，可继续在下方浏览目录。");
    } catch (e) {
      if (!controller.signal.aborted) setNotice((e as Error).message);
    } finally {
      setNativeBusy(false);
    }
  }
  return (
    <Modal
      title="选择本地文件夹"
      description="浏览这台设备上的目录，或打开系统文件夹选择窗口。"
      onClose={onClose}
      wide
    >
      <div className="form-body directory-browser">
        <div className="directory-shortcuts">
          <button
            type="button"
            onClick={() => void pickNative()}
            disabled={nativeBusy}
          >
            {nativeBusy ? (
              <LoaderCircle className="spin" size={16} />
            ) : (
              <FolderOpen size={16} />
            )}
            {nativeBusy ? "请在系统窗口中选择…" : "打开系统选择器"}
          </button>
          {listing?.shortcuts.map((item) => (
            <button
              type="button"
              key={item.path}
              disabled={nativeBusy}
              onClick={() => void browse(item.path)}
            >
              {item.name}
            </button>
          ))}
        </div>
        {notice && (
          <p className="warning" role="status">
            {notice}
          </p>
        )}
        <form
          className="directory-location"
          onSubmit={(event) => {
            event.preventDefault();
            event.stopPropagation();
            void browse(path);
          }}
        >
          <label htmlFor="directory-location">当前目录</label>
          <div className="directory-input">
            <input
              id="directory-location"
              value={path}
              disabled={nativeBusy || loading}
              onChange={(event) => setPath(event.target.value)}
              placeholder="输入绝对路径后跳转"
            />
            <button type="submit" disabled={loading || nativeBusy}>
              跳转
            </button>
          </div>
        </form>
        {error && (
          <div className="warning" role="alert">
            {error}
            <button type="button" onClick={() => void browse("")}>
              返回用户目录
            </button>
          </div>
        )}
        <div className="directory-heading">
          <code>{listing?.path ?? "正在读取目录…"}</code>
          <button
            type="button"
            disabled={
              !listing ||
              listing.parent === listing.path ||
              loading ||
              nativeBusy
            }
            onClick={() => listing && void browse(listing.parent)}
          >
            <ArrowUp size={14} />
            上一级
          </button>
        </div>
        <div className="directory-list" aria-busy={loading}>
          {loading ? (
            <p role="status">
              <LoaderCircle className="spin" size={18} />
              正在读取文件夹…
            </p>
          ) : listing?.directories.length ? (
            listing.directories.map((item) => (
              <button
                type="button"
                key={item.path}
                disabled={nativeBusy}
                onClick={() => void browse(item.path)}
              >
                <Folder size={18} />
                <span>{item.name}</span>
                <span className="muted">打开</span>
              </button>
            ))
          ) : (
            <p>没有可浏览的子文件夹，可选择当前目录。</p>
          )}
        </div>
        {listing && !listing.selectable && (
          <p className="field-error">
            请选择独立的素材目录，不能选择磁盘根或工具状态目录。
          </p>
        )}
        <small>
          仅列出普通文件夹；隐藏目录可通过路径跳转。不支持符号链接或网络映射盘。
        </small>
      </div>
      <div className="modal-actions">
        <button type="button" onClick={onClose}>
          取消
        </button>
        <button
          type="button"
          className="primary"
          disabled={
            loading || nativeBusy || !listing?.selectable || Boolean(error)
          }
          onClick={() => listing && onSelect(listing.path)}
        >
          选择此文件夹
        </button>
      </div>
    </Modal>
  );
}
