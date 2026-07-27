interface ErrorStateProps {
  title?: string;
  message: string;
}

export function ErrorState({ title = "请求失败", message }: ErrorStateProps) {
  return (
    <div role="alert" className="border border-[color:var(--danger)] bg-[color:var(--danger-subtle)] px-4 py-3 text-[color:var(--danger)] [border-radius:var(--radius-md)]">
      <p className="font-semibold">{title}</p>
      <p>{message}</p>
    </div>
  );
}
