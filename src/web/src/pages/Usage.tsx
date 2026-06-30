import { useState } from 'react';
import { Card, Form, Input, InputNumber, Button, Space } from 'antd';
import { listUsageEvents } from '../api/client';
import { useFetch } from '../hooks/useFetch';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import { formatTime } from '../utils/format';
import type { UsageEvent, UsageEventFilter } from '../api/types';

function Usage() {
  const [filter, setFilter] = useState<UsageEventFilter>({ limit: 100 });
  const [form] = Form.useForm();

  const { data, loading, reload } = useFetch<UsageEvent[]>(
    () => listUsageEvents(filter),
    [filter],
  );

  const onSearch = () => {
    setFilter({ ...form.getFieldsValue(), limit: form.getFieldValue('limit') || 100 });
  };

  return (
    <PageContainer description="按模型/账号统计的 token 用量">
      <Card size="small" style={{ marginBottom: 16 }}>
        <Form form={form} layout="inline" initialValues={{ limit: 100 }}>
          <Form.Item name="provider">
            <Input placeholder="provider" allowClear />
          </Form.Item>
          <Form.Item name="model">
            <Input placeholder="model" allowClear />
          </Form.Item>
          <Form.Item name="account_id">
            <Input placeholder="account_id" allowClear />
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

      <CrudTable<UsageEvent>
        data={data}
        loading={loading}
        onReload={reload}
        emptyText="暂无用量数据"
        columns={[
          {
            title: '时间',
            dataIndex: 'created_at',
            sorter: (a, b) => a.created_at.localeCompare(b.created_at),
            defaultSortOrder: 'descend',
            render: (v: string) => formatTime(v),
          },
          { title: 'Provider', dataIndex: 'provider' },
          { title: '模型', dataIndex: 'model' },
          { title: 'Account', dataIndex: 'account_id' },
          {
            title: 'Tokens',
            dataIndex: 'tokens_used',
            sorter: (a, b) => a.tokens_used - b.tokens_used,
          },
          { title: '请求数', dataIndex: 'requests_count' },
        ]}
      />
    </PageContainer>
  );
}

export default Usage;
