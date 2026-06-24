import { useState } from 'react';
import { App, Button, Form, Input, Modal, Popconfirm, Space, Table } from 'antd';
import {
  deleteModelMapping,
  listProviderModelMappings,
  upsertProviderModelMapping,
} from '../../api/client';
import { useFetch } from '../../hooks/useFetch';
import type { ModelMapping, Provider } from '../../api/types';

// 模型映射管理：provider 的 1:N 子资源，列出/新增/删除。
export default function ModelMappingsModal({
  provider,
  onClose,
}: {
  provider: Provider;
  onClose: () => void;
}) {
  const { message } = App.useApp();
  const { data: mappings, loading, reload } = useFetch<ModelMapping[]>(
    () => listProviderModelMappings(provider.id),
    [provider.id],
  );
  const [form] = Form.useForm();
  const [saving, setSaving] = useState(false);

  const add = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      await upsertProviderModelMapping(provider.id, v);
      form.resetFields();
      message.success('已添加映射');
      reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '添加失败');
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await deleteModelMapping(id);
      message.success('已删除');
      reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '删除失败');
    }
  };

  return (
    <Modal title={`模型映射 — ${provider.name}`} open footer={null} onCancel={onClose} destroyOnClose>
      <Form form={form} layout="vertical">
        <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
          <Form.Item name="from_model" noStyle rules={[{ required: true }]}>
            <Input placeholder="客户端模型名 (from)" />
          </Form.Item>
          <Form.Item name="to_model" noStyle rules={[{ required: true }]}>
            <Input placeholder="上游模型名 (to)" />
          </Form.Item>
          <Button type="primary" loading={saving} onClick={add}>
            添加
          </Button>
        </Space.Compact>
      </Form>
      <Table<ModelMapping>
        size="small"
        rowKey="id"
        loading={loading}
        dataSource={mappings || []}
        pagination={false}
        locale={{ emptyText: '暂无映射' }}
        columns={[
          { title: '客户端模型', dataIndex: 'from_model' },
          { title: '上游模型', dataIndex: 'to_model' },
          {
            title: '',
            width: 60,
            render: (_: unknown, r: ModelMapping) => (
              <Popconfirm title="删除该映射？" onConfirm={() => remove(r.id)}>
                <a style={{ color: '#ff4d4f' }}>删除</a>
              </Popconfirm>
            ),
          },
        ]}
      />
    </Modal>
  );
}
