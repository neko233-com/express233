import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type ReactNode,
  type Ref,
} from "react";
import { ChevronRight, Info, SquareTerminal, X } from "lucide-react";
import { errorMessage } from "./api";

type Tone = "primary" | "secondary" | "ghost" | "danger";
interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  tone?: Tone;
  small?: boolean;
}
export function Button({
  tone = "secondary",
  small,
  className = "",
  type = "button",
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={`btn btn-${tone}${small ? " btn-sm" : ""} ${className}`}
      {...props}
    />
  );
}

export function Wordmark() {
  return (
    <div className="wordmark">
      <SquareTerminal size={25} aria-hidden="true" />
      <span>Express233</span>
    </div>
  );
}

export function PageHeader({
  title,
  description,
  children,
}: {
  title: string;
  description?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <header className="page-header">
      <div>
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {children && <div className="toolbar">{children}</div>}
    </header>
  );
}

export function Card({
  title,
  description,
  actions,
  children,
  className = "",
  id,
}: {
  title?: string;
  description?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
  className?: string;
  id?: string;
}) {
  return (
    <section id={id} className={`card ${className}`}>
      {(title || actions) && (
        <header className="card-head">
          <div>
            {title && <h2>{title}</h2>}
            {description && <p className="hint">{description}</p>}
          </div>
          {actions && <div className="toolbar">{actions}</div>}
        </header>
      )}
      {children}
    </section>
  );
}

export function Field({
  label,
  children,
  hint,
}: {
  label: string;
  children: ReactNode;
  hint?: string;
}) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
      {hint && <span className="hint">{hint}</span>}
    </label>
  );
}

export function Badge({
  children,
  kind = "neutral",
}: {
  children: ReactNode;
  kind?: "neutral" | "ok" | "warn" | "danger" | "info";
}) {
  return <span className={`badge badge-${kind}`}>{children}</span>;
}

export function VersionBadge({ status, testId }: { status: string; testId?: string }) {
  return (
    <span data-testid={testId}>
      <Badge
        kind={
          status === "published"
            ? "ok"
            : status === "rejected"
              ? "danger"
              : status === "pending_review"
                ? "warn"
                : "neutral"
        }
      >
        {status}
      </Badge>
    </span>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return <div className="empty">{children}</div>;
}

export function ResourceNotice({
  loading,
  error,
  retry,
}: {
  loading?: boolean;
  error?: string;
  retry?: () => void;
}) {
  if (error)
    return (
      <div className="notice error" role="alert">
        <Info size={16} />
        <span>{error}</span>
        {retry && (
          <Button small onClick={retry}>
            重试
          </Button>
        )}
      </div>
    );
  return loading ? (
    <p className="hint" role="status">
      正在加载…
    </p>
  ) : null;
}

export function Table({
  headers,
  children,
  testId,
  id,
}: {
  headers: string[];
  children?: ReactNode;
  testId?: string;
  id?: string;
}) {
  return (
    <div className="table-wrap" data-testid={testId}>
      <table className="data-table" id={id}>
        <thead>
          <tr>
            {headers.map((name, i) => (
              <th key={i} scope="col">
                {name}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}

export function Disclosure({
  title,
  children,
  id,
  testId,
  badge,
  ref,
}: {
  title: string;
  children: ReactNode;
  id?: string;
  testId?: string;
  badge?: ReactNode;
  ref?: Ref<HTMLDetailsElement>;
}) {
  return (
    <details className="card disclosure" id={id} data-testid={testId} ref={ref}>
      <summary>
        <ChevronRight size={15} aria-hidden="true" />
        <strong>{title}</strong>
        {badge}
      </summary>
      <div className="disclosure-body">{children}</div>
    </details>
  );
}

type ToastKind = "ok" | "error" | "warn";
type DialogOptions = {
  title: string;
  message: string;
  confirmText?: string;
  danger?: boolean;
  value?: string;
  secret?: boolean;
  onlyClose?: boolean;
};
type DialogRequest = DialogOptions & { finish: (result: string | null) => void };
interface Feedback {
  toast: (message: string, kind?: ToastKind) => void;
  confirm: (options: DialogOptions) => Promise<boolean>;
  prompt: (options: DialogOptions) => Promise<string | null>;
}
const FeedbackContext = createContext<Feedback | null>(null);

export function FeedbackProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<{ id: number; message: string; kind: ToastKind }[]>([]);
  const [dialog, setDialog] = useState<DialogRequest | null>(null);
  const sequence = useRef(0);
  const modalPending = useRef(false);
  const toast: Feedback["toast"] = (message, kind = "ok") => {
    const id = ++sequence.current;
    setToasts((list) => [...list.slice(-2), { id, message, kind }]);
    window.setTimeout(() => setToasts((list) => list.filter((item) => item.id !== id)), 3500);
  };
  const open = (options: DialogOptions) =>
    new Promise<string | null>((resolve) => {
      if (modalPending.current) {
        resolve(null);
        return;
      }
      modalPending.current = true;
      setDialog({
        ...options,
        finish: (result) => {
          modalPending.current = false;
          setDialog(null);
          resolve(result);
        },
      });
    });
  return (
    <FeedbackContext
      value={{ toast, confirm: async (options) => (await open(options)) !== null, prompt: open }}
    >
      {children}
      <div className="toasts" aria-live="polite">
        {toasts.map(({ id, kind, message }) => (
          <div key={id} className={`toast ${kind}`} role={kind === "error" ? "alert" : "status"}>
            <span>{message}</span>
            <Button
              tone="ghost"
              small
              aria-label="关闭通知"
              onClick={() => setToasts((list) => list.filter((item) => item.id !== id))}
            >
              <X size={15} />
            </Button>
          </div>
        ))}
      </div>
      {dialog && <ConfirmDialog dialog={dialog} />}
    </FeedbackContext>
  );
}

function ConfirmDialog({ dialog }: { dialog: DialogRequest }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog
      ref={ref}
      className="modal-card"
      aria-labelledby="modal-title"
      onCancel={(event) => {
        event.preventDefault();
        dialog.finish(null);
      }}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          dialog.finish(String(new FormData(event.currentTarget).get("value") ?? ""));
        }}
      >
        <h2 id="modal-title">{dialog.title}</h2>
        <p className="modal-message">{dialog.message}</p>
        {dialog.value !== undefined && (
          <input
            className="input"
            name="value"
            aria-label={dialog.title}
            type={dialog.secret ? "password" : "text"}
            defaultValue={dialog.value}
            autoComplete="off"
            autoFocus
          />
        )}
        <div className="toolbar modal-actions">
          {!dialog.onlyClose && (
            <Button autoFocus={dialog.value === undefined} onClick={() => dialog.finish(null)}>
              取消
            </Button>
          )}
          <Button
            type="submit"
            data-modal="ok"
            autoFocus={dialog.onlyClose}
            tone={dialog.danger ? "danger" : "primary"}
          >
            {dialog.confirmText || "确认"}
          </Button>
        </div>
      </form>
    </dialog>
  );
}

export function useFeedback() {
  const feedback = useContext(FeedbackContext);
  if (!feedback) throw new Error("FeedbackProvider is required");
  return feedback;
}

export function useAction() {
  const { toast } = useFeedback();
  const lock = useRef(false);
  const [busy, setBusy] = useState(false);
  async function run(work: () => Promise<void>) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    try {
      await work();
    } catch (error) {
      toast(errorMessage(error), "error");
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  return { busy, run };
}
