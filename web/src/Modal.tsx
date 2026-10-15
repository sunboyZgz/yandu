import { useEffect, useId, useRef, type ReactNode } from "react";
import { X } from "lucide-react";

const dialogs: HTMLElement[] = [];
export default function Modal({
  title,
  description,
  children,
  onClose,
  wide = false,
  busy = false,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
  busy?: boolean;
}) {
  const ref = useRef<HTMLElement>(null);
  const close = useRef(onClose);
  const titleId = useId();
  useEffect(() => {
    close.current = () => {
      if (!busy) onClose();
    };
  }, [onClose, busy]);
  useEffect(() => {
    const dialog = ref.current!;
    const previous = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    dialogs.push(dialog);
    const focusable = () =>
      Array.from(
        dialog.querySelectorAll<HTMLElement>(
          'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], [tabindex="0"]',
        ),
      ).filter((el) => el.getClientRects().length > 0);
    if (!dialog.contains(document.activeElement))
      (focusable()[0] ?? dialog).focus();
    const keydown = (event: KeyboardEvent) => {
      if (dialogs.at(-1) !== dialog) return;
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        close.current();
      }
      if (event.key === "Tab") {
        const items = focusable();
        const next = event.shiftKey ? items.at(-1) : items[0];
        if (
          !items.length ||
          document.activeElement ===
            (event.shiftKey ? items[0] : items.at(-1)) ||
          !dialog.contains(document.activeElement)
        ) {
          event.preventDefault();
          (next ?? dialog).focus();
        }
      }
    };
    document.addEventListener("keydown", keydown);
    return () => {
      dialogs.splice(dialogs.indexOf(dialog), 1);
      document.removeEventListener("keydown", keydown);
      document.body.style.overflow = overflow;
      if (previous?.isConnected) previous.focus();
    };
  }, []);
  return (
    <div className="modal-backdrop" onClick={() => close.current()}>
      <section
        ref={ref}
        className={"modal " + (wide ? "wide" : "")}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? `${titleId}-description` : undefined}
        tabIndex={-1}
        onClick={(event) => event.stopPropagation()}
      >
        <header>
          <div>
            <h2 id={titleId}>{title}</h2>
            {description && <p id={`${titleId}-description`}>{description}</p>}
          </div>
          <button
            type="button"
            className="icon-button"
            aria-label="关闭"
            disabled={busy}
            onClick={() => close.current()}
          >
            <X size={20} />
          </button>
        </header>
        {children}
      </section>
    </div>
  );
}
