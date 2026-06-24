import { Form, Input } from 'antd';
import { listRoles, upsertRole } from '../api/client';
import { useCrud } from '../hooks/useCrud';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import EntityFormModal from '../components/EntityFormModal';
import type { Role } from '../api/types';

function Roles() {
  const crud = useCrud<Role>({
    list: () => listRoles(),
    save: upsertRole,
    deps: [],
    defaults: {} as Partial<Role>,
  });

  return (
    <PageContainer description="定义后台角色与权限">
      <CrudTable<Role>
        data={crud.data}
        loading={crud.loading}
        onReload={crud.reload}
        onCreate={crud.openCreate}
        createText="新增角色"
        columns={[
          { title: '名称', dataIndex: 'name', sorter: (a, b) => a.name.localeCompare(b.name) },
          { title: '描述', dataIndex: 'description' },
          { title: '权限', dataIndex: 'permissions' },
          {
            title: '操作',
            fixed: 'right',
            render: (_, r) => <a onClick={() => crud.openEdit(r)}>编辑</a>,
          },
        ]}
      />

      <EntityFormModal
        title={crud.isEditing ? '编辑角色' : '新增角色'}
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
        <Form.Item label="描述" name="description">
          <Input />
        </Form.Item>
        <Form.Item label="权限 (JSON 或逗号分隔)" name="permissions">
          <Input.TextArea rows={2} placeholder='["read","write"]' />
        </Form.Item>
      </EntityFormModal>
    </PageContainer>
  );
}

export default Roles;
