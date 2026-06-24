import { Tag } from 'antd';

const COLORS: Record<string, string> = {
  active: 'success',
  healthy: 'success',
  disabled: 'default',
  inactive: 'default',
  expired: 'error',
  not_found: 'error',
  rate_limited: 'warning',
  quota_exceeded: 'warning',
  auth_failed: 'error',
};

function StatusBadge({ status }: { status?: string }) {
  if (!status) return <Tag>—</Tag>;
  return <Tag color={COLORS[status] ?? 'default'}>{status}</Tag>;
}

export default StatusBadge;
