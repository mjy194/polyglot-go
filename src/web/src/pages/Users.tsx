import { useState } from 'react';
import { Form, Input, Modal, Tag, Typography, Space, App } from 'antd';
import { listUsers, upsertUser, listUserRoles, assignRole, listRoles } from '../api/client';
import { useCrud } from '../hooks/useCrud';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import EntityFormModal from '../components/EntityFormModal';
import StatusBadge from '../components/StatusBadge';
import type { User, Role } from '../api/types';

function Users() {
  const { message } = App.useApp();
  const crud = useCrud<User>({
    list: () => listUsers(),
    save: upsertUser,
    deps: [],
    defaults: { status: 'active' } as Partial<User>,
  });

  // 角色弹窗
  const [roleOpen, setRoleOpen] = useState(false);
  const [roleUser, setRoleUser] = useState<User>();
  const [userRoles, setUserRoles] = useState<Role[]>([]);
  const [allRoles, setAllRoles] = useState<Role[]>([]);
  const [roleForm] = Form.useForm();

  const openRoles = async (u: User) => {
    setRoleUser(u);
    setRoleOpen(true);
    roleForm.resetFields();
    try {
      const [ur, all] = await Promise.all([listUserRoles(u.id), listRoles()]);
      setUserRoles((ur as unknown as Role[]) || []);
      setAllRoles(all || []);
    } catch (e: any) {
      message.error(e?.response?.data?.error || '加载角色失败');
    }
  };

  const onAssign = async () => {
    const { role_id } = await roleForm.validateFields();
    if (!roleUser) return;
    try {
      await assignRole({ user_id: roleUser.id, role_id });
      message.success('已分配');
      const ur = await listUserRoles(roleUser.id);
      setUserRoles((ur as unknown as Role[]) || []);
      roleForm.resetFields();
    } catch (e: any) {
      message.error(e?.response?.data?.error || '分配失败');
    }
  };

  return (
    <PageContainer description="管理后台用户及其角色">
      <CrudTable<User>
        data={crud.data}
        loading={crud.loading}
        onReload={crud.reload}
        onCreate={crud.openCreate}
        createText="新增用户"
        columns={[
          { title: '邮箱', dataIndex: 'email', sorter: (a, b) => a.email.localeCompare(b.email) },
          { title: '昵称', dataIndex: 'display_name' },
          { title: '状态', dataIndex: 'status', render: (s: string) => <StatusBadge status={s} /> },
          {
            title: '操作',
            fixed: 'right',
            render: (_, r) => (
              <Space>
                <a onClick={() => crud.openEdit(r)}>编辑</a>
                <a onClick={() => openRoles(r)}>角色</a>
              </Space>
            ),
          },
        ]}
      />

      <EntityFormModal
        title={crud.isEditing ? '编辑用户' : '新增用户'}
        open={crud.open}
        form={crud.form}
        saving={crud.saving}
        onOk={crud.submit}
        onCancel={crud.closeModal}
      >
        <Form.Item name="id" hidden>
          <Input />
        </Form.Item>
        <Form.Item label="邮箱" name="email" rules={[{ required: true, type: 'email' }]}>
          <Input />
        </Form.Item>
        <Form.Item label="昵称" name="display_name">
          <Input />
        </Form.Item>
        <Form.Item label="状态" name="status">
          <Input placeholder="active / disabled" />
        </Form.Item>
      </EntityFormModal>

      <Modal
        title={`角色 — ${roleUser?.email || ''}`}
        open={roleOpen}
        onCancel={() => setRoleOpen(false)}
        footer={null}
        destroyOnClose
      >
        <div style={{ marginBottom: 12 }}>
          {userRoles.length === 0 ? (
            <Typography.Text type="secondary">该用户暂无角色</Typography.Text>
          ) : (
            userRoles.map((r) => (
              <Tag key={r.id} color="blue">
                {r.name}
              </Tag>
            ))
          )}
        </div>
        <Form form={roleForm} onFinish={onAssign}>
          <Form.Item name="role_id" rules={[{ required: true, message: '输入 role_id' }]}>
            <Input.Search placeholder="role_id" enterButton="分配" onSearch={onAssign} />
          </Form.Item>
        </Form>
        <Typography.Paragraph type="secondary" style={{ marginTop: 8 }}>
          可用角色:{allRoles.map((r) => `${r.name}(${r.id})`).join(', ') || '无'}
        </Typography.Paragraph>
      </Modal>
    </PageContainer>
  );
}

export default Users;
