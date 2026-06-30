// 统一时间格式化：本地时间，精确到秒（YYYY-MM-DD HH:mm:ss）。
// 入参是后端返回的 ISO 字符串；空值返回 "—"。
export function formatTime(v?: string | null): string {
  if (!v) return '—';
  const d = new Date(v);
  if (isNaN(d.getTime())) return v;
  const pad = (n: number) => String(n).padStart(2, '0');
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ` +
    `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  );
}
