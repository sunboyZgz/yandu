import type { ReactNode } from "react";

export function Required() {
  return <span className="required-mark">必填</span>;
}

export function Field({
  id,
  label,
  required,
  hint,
  error,
  children,
}: {
  id: string;
  label: string;
  required?: boolean;
  hint?: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className="form-field">
      <label htmlFor={id}>
        {label}
        {required && <Required />}
      </label>
      {children}
      {hint && <small id={`${id}-hint`}>{hint}</small>}
      {error && (
        <span className="field-error" id={`${id}-error`}>
          {error}
        </span>
      )}
    </div>
  );
}

export function feedback(id: string, error?: string) {
  return {
    "aria-invalid": Boolean(error),
    "aria-describedby": `${id}-hint${error ? ` ${id}-error` : ""}`,
  };
}

export function focusFirstError(form: HTMLFormElement) {
  requestAnimationFrame(() =>
    form.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus(),
  );
}

export const projectIdError = (value: string) =>
  /^[a-z][a-z0-9-]{0,47}$/.test(value)
    ? ""
    : "以小写字母开头，仅支持小写字母、数字和连字符，最多 48 位";
export const projectNameError = (value: string) =>
  !value.trim()
    ? "请填写项目名称"
    : new TextEncoder().encode(value.trim()).length > 160
      ? "项目名称过长，请缩短至 160 字节以内（中文约 53 字）"
      : "";
export const directoryError = (value: string) =>
  !value.trim()
    ? "请选择本地文件夹"
    : /^(?:\/(?!\/)|[a-zA-Z]:[\\/])/.test(value.trim()) &&
        !/[\r\n\0]/.test(value)
      ? ""
      : "请使用绝对路径，例如 /Users/name/media 或 D:/media";
export const directoryNameError = (value: string) =>
  !value.trim()
    ? "请填写目录名称"
    : /[\\/<>:"|?*\x00-\x1f]/.test(value) ||
        /[. ]$/.test(value) ||
        /^(\.{1,2}|CON|PRN|AUX|NUL|CLOCK\$|COM[0-9]|LPT[0-9])(?:\.|$)/i.test(
          value,
        )
      ? "目录名不能包含路径分隔符、特殊字符或系统保留名称"
      : "";
