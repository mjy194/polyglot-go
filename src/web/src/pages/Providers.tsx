import { Form, Input, Select } from 'antd';
import { listProviders, upsertProvider } from '../api/client';
import { useCrud } from '../hooks/useCrud';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import EntityFormModal from '../components/EntityFormModal';
import StatusBadge from '../components/StatusBadge';
import type { Provider } from '../api/types';

function Providers() {
  const crud = useCrud<Provider>({
    list: () => listProviders(),
    save: upsertProvider,
    deps: [],
    defaults: { status: 'active', auth_type: 'bearer' } as Partial<Provider>,
  });

  return (
    <PageContainer description="管理上游 AI 服务提供方">
      <CrudTable<Provider>
        data={crud.data}
        loading={crud.loading}
        onReload={crud.reload}
        onCreate={crud.openCreate}
        createText="新增 Provider"
        columns={[
          { title: '名称', dataIndex: 'name', sorter: (a, b) => a.name.localeCompare(b.name) },
          { title: '类型', dataIndex: 'type' },
          { title: 'Base URL', dataIndex: 'base_url' },
          { title: '认证', dataIndex: 'auth_type' },
          {
            title: '状态',
            dataIndex: 'status',
            render: (s: string) => <StatusBadge status={s} />,
          },
          {
            title: '操作',
            fixed: 'right',
            render: (_, r) => <a onClick={() => crud.openEdit(r)}>编辑</a>,
          },
        ]}
      />

      <EntityFormModal
        title={crud.isEditing ? '编辑 Provider' : '新增 Provider'}
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
        <Form.Item label="类型" name="type" rules={[{ required: true }]}>
          <Select
            options={[
              { value: 'anthropic', label: 'anthropic' },
              { value: 'openai', label: 'openai' },
              { value: 'gemini', label: 'gemini' },
              { value: 'uipath', label: 'uipath' },
            ]}
          />
        </Form.Item>
        <Form.Item label="Base URL" name="base_url">
          <Input placeholder="https://..." />
        </Form.Item>
        <Form.Item label="认证类型" name="auth_type">
          <Input placeholder="bearer / api_key / oauth" />
        </Form.Item>
        <Form.Item label="默认 Headers (JSON)" name="default_headers">
          <Input.TextArea rows={2} placeholder='{"x-foo":"bar"}' />
        </Form.Item>
        <Form.Item label="状态" name="status">
          <Select
            options={[
              { value: 'active', label: 'active' },
              { value: 'disabled', label: 'disabled' },
            ]}
          />
        </Form.Item>
      </EntityFormModal>
    </PageContainer>
  );
}

export default Providers;
