import { useState } from 'react';
import { Card, Tag, Form, Input, InputNumber, Button, Space } from 'antd';
import { listRequestLogs } from '../api/client';
import { useFetch } from '../hooks/useFetch';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import { formatTime } from '../utils/format';
import type { RequestLog, RequestLogFilter } from '../api/types';

function Logs() {
  const [filter, setFilter] = useState<RequestLogFilter>({ limit: 100 });
  const [form] = Form.useForm();

  const { data, loading, reload } = useFetch<RequestLog[]>(
    () => listRequestLogs(filter),
    [filter],
  );

  const onSearch = () => {
    setFilter({ ...form.getFieldsValue(), limit: form.getFieldValue('limit') || 100 });
  };

  return (
    <PageContainer description="网关请求审计日志">
      <Card size="small" style={{ marginBottom: 16 }}>
        <Form form={form} layout="inline" initialValues={{ limit: 100 }}>
          <Form.Item name="provider">
            <Input placeholder="provider" allowClear />
          </Form.Item>
          <Form.Item name="protocol">
            <Input placeholder="protocol" allowClear />
          </Form.Item>
          <Form.Item name="user_id">
            <Input placeholder="user_id" allowClear />
          </Form.Item>
          <Form.Item name="limit">
            <InputNumber min={1} max={1000} placeholder="limit" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" onClick={onSearch}>
                查询
              </Button>
              <Button onClick={() => form.resetFields()}>重置</Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>

      <CrudTable<RequestLog>
        data={data}
        loading={loading}
        onReload={reload}
        emptyText="暂无请求日志"
        columns={[
          {
            title: '用户',
            render: (_, r) => r.user_name || r.user_id || '—',
          },
          {
            title: 'API 密钥',
            render: (_, r) => r.api_key_name || r.api_key_id || '—',
          },
          {
            title: '账户',
            dataIndex: 'account_id',
            render: (v: string) => v || '—',
          },
          { title: '模型', dataIndex: 'model', render: (v: string) => v || '—' },
          { title: '端点', dataIndex: 'endpoint', render: (v: string) => v || '—' },
          {
            title: '分组',
            dataIndex: 'group',
            render: (v: string) => v || '—',
          },
          {
            title: '类型',
            dataIndex: 'type',
            render: (v: string) => {
              if (!v) return '—';
              return <Tag color={v === 'stream' ? 'blue' : 'default'}>{v === 'stream' ? '流式' : '非流式'}</Tag>;
            },
          },
          {
            title: '计费模式',
            render: () => '—',
          },
          {
            title: 'Token (上游/下游/缓存)',
            render: (_, r) => `${r.input_tokens || 0} / ${r.output_tokens || 0} / ${r.cached_tokens || 0}`,
          },
          {
            title: '费用',
            dataIndex: 'cost',
            render: (v: number) => (v ? v.toFixed(4) : '—'),
          },
          {
            title: '首 Token',
            dataIndex: 'ttft_ms',
            render: (v: number) => (v ? `${v} ms` : '—'),
          },
          {
            title: '耗时',
            dataIndex: 'latency_ms',
            sorter: (a, b) => a.latency_ms - b.latency_ms,
            render: (v: number) => `${v} ms`,
          },
          {
            title: '时间',
            dataIndex: 'created_at',
            defaultSortOrder: 'descend',
            render: (v: string) => formatTime(v),
          },
          {
            title: 'IP',
            dataIndex: 'client_ip',
            render: (v: string) => v || '—',
          },
          {
            title: '状态',
            dataIndex: 'status_code',
            render: (code: number, r) => <Tag color={r.success ? 'success' : 'error'}>{code}</Tag>,
          },
        ]}
      />
    </PageContainer>
  );
}

export default Logs;
