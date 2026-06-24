import { useState } from 'react';
import { Form, Input, Alert, Typography } from 'antd';
import { listApiKeys, upsertApiKey } from '../api/client';
import { useCrud } from '../hooks/useCrud';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import EntityFormModal from '../components/EntityFormModal';
import StatusBadge from '../components/StatusBadge';
import type { APIKey } from '../api/types';

const { Text } = Typography;

function ApiKeys() {
  const [newKey, setNewKey] = useState<string>();

  const crud = useCrud<APIKey>({
    list: () => listApiKeys(),
    save: async (values) => {
      const saved = (await upsertApiKey(values)) as APIKey;
      // 仅新建(无 id)且后端返回明文 key 时展示一次
      if (!values.id && saved.key) setNewKey(saved.key);
      return saved;
    },
    deps: [],
    defaults: { status: 'active', scopes: 'openai,anthropic,responses,gemini' } as Partial<APIKey>,
  });

  return (
    <PageContainer description="管理调用网关用的 API Key(下游程序凭证)">
      {newKey && (
        <Alert
          type="success"
          showIcon
          closable
          onClose={() => setNewKey(undefined)}
          style={{ marginBottom: 16 }}
          message="新 API Key 已生成(仅此一次可见,请立即复制)"
          description={
            <Text copyable code>
              {newKey}
            </Text>
          }
        />
      )}

      <CrudTable<APIKey>
        data={crud.data}
        loading={crud.loading}
        onReload={crud.reload}
        onCreate={crud.openCreate}
        createText="新增 Key"
        columns={[
          { title: '名称', dataIndex: 'name', sorter: (a, b) => a.name.localeCompare(b.name) },
          {
            title: 'Key',
            dataIndex: 'key',
            render: (k?: string) => (k ? <Text code>{k}</Text> : '—'),
          },
          { title: 'Scopes', dataIndex: 'scopes' },
          { title: '状态', dataIndex: 'status', render: (s: string) => <StatusBadge status={s} /> },
          { title: '最后使用', dataIndex: 'last_used_at', render: (v?: string) => v || '—' },
          {
            title: '操作',
            fixed: 'right',
            render: (_, r) => <a onClick={() => crud.openEdit(r)}>编辑</a>,
          },
        ]}
      />

      <EntityFormModal
        title={crud.isEditing ? '编辑 API Key' : '新增 API Key'}
        open={crud.open}
        form={crud.form}
        saving={crud.saving}
        onOk={crud.submit}
        onCancel={crud.closeModal}
      >
        <Text type="secondary">新建时「Key」留空,后端会自动生成 pk_... 并返回一次明文。</Text>
        <Form.Item name="id" hidden>
          <Input />
        </Form.Item>
        <Form.Item label="名称" name="name" rules={[{ required: true }]}>
          <Input />
        </Form.Item>
        <Form.Item label="User ID(可选)" name="user_id">
          <Input />
        </Form.Item>
        <Form.Item label="Key(留空自动生成)" name="key">
          <Input placeholder="pk_..." />
        </Form.Item>
        <Form.Item label="Scopes" name="scopes">
          <Input placeholder="openai,anthropic,responses,gemini (逗号分隔或 JSON 数组)" />
        </Form.Item>
        <Form.Item label="状态" name="status">
          <Input placeholder="active / disabled" />
        </Form.Item>
      </EntityFormModal>
    </PageContainer>
  );
}

export default ApiKeys;
