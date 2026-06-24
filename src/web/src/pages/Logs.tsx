import { useState } from 'react';
import { Card, Tag, Form, Input, InputNumber, Button, Space } from 'antd';
import { listRequestLogs } from '../api/client';
import { useFetch } from '../hooks/useFetch';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
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
            title: '时间',
            dataIndex: 'created_at',
            sorter: (a, b) => a.created_at.localeCompare(b.created_at),
            defaultSortOrder: 'descend',
          },
          { title: 'Provider', dataIndex: 'provider' },
          { title: '协议', dataIndex: 'protocol' },
          { title: '模型', dataIndex: 'model' },
          {
            title: '状态',
            dataIndex: 'status_code',
            render: (code: number, r) => <Tag color={r.success ? 'success' : 'error'}>{code}</Tag>,
          },
          {
            title: '延迟(ms)',
            dataIndex: 'latency_ms',
            sorter: (a, b) => a.latency_ms - b.latency_ms,
          },
          { title: '入', dataIndex: 'input_tokens' },
          { title: '出', dataIndex: 'output_tokens' },
          { title: '错误', dataIndex: 'error_message', ellipsis: true },
        ]}
      />
    </PageContainer>
  );
}

export default Logs;
