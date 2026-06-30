import { Form, Input, Popconfirm, Select, Space, Switch, Typography } from 'antd';
import { App } from 'antd';
import { deleteApiKey, listApiKeys, upsertApiKey } from '../api/client';
import { useCrud } from '../hooks/useCrud';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import EntityFormModal from '../components/EntityFormModal';
import { formatTime } from '../utils/format';
import type { APIKey } from '../api/types';

const { Text } = Typography;

function ApiKeys() {
  const { message } = App.useApp();

  const crud = useCrud<APIKey>({
    list: () => listApiKeys(),
    save: upsertApiKey,
    deps: [],
    defaults: { status: 'active', scopes: 'openai,anthropic,responses,gemini' } as Partial<APIKey>,
  });

  const toggleStatus = async (key: APIKey, active: boolean) => {
    try {
      await upsertApiKey({ ...key, status: active ? 'active' : 'disabled' });
      message.success(active ? '已启用' : '已禁用');
      crud.reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '更新失败');
    }
  };

  const remove = async (key: APIKey) => {
    try {
      await deleteApiKey(key.id);
      message.success('已删除');
      crud.reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '删除失败');
    }
  };

  return (
    <PageContainer description="我自己的 API Key(调用网关的凭证,归属当前登录用户)">
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
            render: (k?: string) => (k ? <Text copyable code>{k}</Text> : '—'),
          },
          { title: 'Scopes', dataIndex: 'scopes' },
          {
            title: '状态',
            dataIndex: 'status',
            width: 80,
            render: (s: string, r: APIKey) => (
              <Switch size="small" checked={s === 'active'} onChange={(v) => toggleStatus(r, v)} />
            ),
          },
          { title: '最后使用', dataIndex: 'last_used_at', render: (v?: string) => formatTime(v) },
          {
            title: '操作',
            fixed: 'right',
            render: (_, r) => (
              <Space>
                <a onClick={() => crud.openEdit(r)}>编辑</a>
                <Popconfirm title="删除该 Key？" onConfirm={() => remove(r)}>
                  <a style={{ color: '#ff4d4f' }}>删除</a>
                </Popconfirm>
              </Space>
            ),
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
        <Form.Item name="id" hidden>
          <Input />
        </Form.Item>
        <Form.Item label="名称" name="name" rules={[{ required: true }]}>
          <Input />
        </Form.Item>
        <Form.Item label="Key(留空自动生成)" name="key" extra="编辑时保留原值；新建留空由后端生成。">
          <Input placeholder="pk_..." />
        </Form.Item>
        <Form.Item label="Scopes" name="scopes">
          <Input placeholder="openai,anthropic,responses,gemini (逗号分隔或 JSON 数组)" />
        </Form.Item>
        <Form.Item label="状态" name="status">
          <Select
            options={[
              { value: 'active', label: '启用 (active)' },
              { value: 'disabled', label: '禁用 (disabled)' },
            ]}
          />
        </Form.Item>
      </EntityFormModal>
    </PageContainer>
  );
}

export default ApiKeys;
