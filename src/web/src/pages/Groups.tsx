import { useEffect, useState } from 'react';
import { Form, Input, InputNumber, Select, Space, Popconfirm, Switch, App, Tag } from 'antd';
import {
  deleteGroup,
  listGroupProviders,
  listGroups,
  listProviders,
  setGroupProviders,
  upsertGroup,
} from '../api/client';
import { useCrud } from '../hooks/useCrud';
import { useFetch } from '../hooks/useFetch';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import EntityFormModal from '../components/EntityFormModal';
import type { Group } from '../api/types';

const STRATEGIES = [
  { value: 'failover', label: 'failover（主备）' },
  { value: 'round_robin', label: 'round_robin（轮询）' },
  { value: 'random', label: 'random（随机）' },
];

function Groups() {
  const { message } = App.useApp();
  const { data: allProviders } = useFetch(listProviders, []);
  const providerOptions = (allProviders || []).map((p) => ({ value: p.id, label: p.name }));
  const crud = useCrud<Group>({
    list: listGroups,
    save: async (values: any) => {
      const { provider_ids: providerIds = [], ...groupValues } = values;
      const saved = await upsertGroup(groupValues);
      await setGroupProviders(
        saved.id,
        (providerIds as string[]).map((id, i) => ({ provider_id: id, priority: i })),
      );
      return saved;
    },
    deps: [],
    defaults: { status: 'active', ratio: 1, strategy: 'failover' } as Partial<Group>,
  });

  const [providerNames, setProviderNames] = useState<Record<string, string[]>>({});
  useEffect(() => {
    const data = crud.data;
    if (!data) return;
    let cancelled = false;
    (async () => {
      const entries = await Promise.all(
        data.map(async (g) => {
          const links = await listGroupProviders(g.id).catch(() => []);
          return [g.id, links.map((l) => l.name).filter(Boolean)] as [string, string[]];
        }),
      );
      if (!cancelled) {
        setProviderNames(Object.fromEntries(entries));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [crud.data]);

  const openEdit = async (group: Group) => {
    await crud.openEdit(group);
    const links = await listGroupProviders(group.id).catch(() => []);
    crud.form.setFieldsValue({ provider_ids: links.map((l) => l.provider_id) });
  };

  const toggleStatus = async (group: Group, active: boolean) => {
    try {
      await upsertGroup({ ...group, status: active ? 'active' : 'disabled' });
      message.success(active ? '已启用' : '已禁用');
      crud.reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '更新失败');
    }
  };

  const remove = async (group: Group) => {
    try {
      await deleteGroup(group.id);
      message.success('已删除');
      crud.reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '删除失败');
    }
  };

  return (
    <PageContainer description="分组管理（访问/计费层：用户/key 属于分组，provider 服务分组）">
      <CrudTable<Group>
        data={crud.data}
        loading={crud.loading}
        onReload={crud.reload}
        onCreate={crud.openCreate}
        createText="新增分组"
        rowKey="id"
        columns={[
          { title: '名称', dataIndex: 'name' },
          { title: '描述', dataIndex: 'description', ellipsis: true },
          { title: '倍率', dataIndex: 'ratio', render: (v: number) => `${v}×` },
          { title: '策略', dataIndex: 'strategy' },
          {
            title: '服务 Provider',
            dataIndex: 'id',
            render: (id: string) => {
              const names = providerNames[id] || [];
              if (!names.length) return <Tag>—</Tag>;
              return (
                <Space size={4} wrap>
                  {names.map((n) => (
                    <Tag key={n} color="blue">
                      {n}
                    </Tag>
                  ))}
                </Space>
              );
            },
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 80,
            render: (s: string, r: Group) => (
              <Switch size="small" checked={s === 'active'} onChange={(v) => toggleStatus(r, v)} />
            ),
          },
          {
            title: '操作',
            fixed: 'right',
            width: 120,
            render: (_: unknown, r: Group) => (
              <Space>
                <a onClick={() => openEdit(r)}>编辑</a>
                {r.name !== 'default' && (
                  <Popconfirm title="删除该分组？" onConfirm={() => remove(r)}>
                    <a style={{ color: '#ff4d4f' }}>删除</a>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />
      <EntityFormModal
        title={crud.isEditing ? '编辑分组' : '新增分组'}
        open={crud.open}
        form={crud.form}
        saving={crud.saving}
        onOk={crud.submit}
        onCancel={crud.closeModal}
      >
        <Form.Item name="id" hidden><Input /></Form.Item>
        <Form.Item label="名称" name="name" rules={[{ required: true }]}>
          <Input placeholder="default / vip / ..." />
        </Form.Item>
        <Form.Item label="描述" name="description">
          <Input />
        </Form.Item>
        <Form.Item label="计费倍率" name="ratio" extra="1 = 原价；1.5 = 1.5 倍计费（后续接定价表）">
          <InputNumber min={0} step={0.1} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item label="选择策略" name="strategy">
          <Select options={STRATEGIES} />
        </Form.Item>
        <Form.Item
          label="服务 Provider"
          name="provider_ids"
          extra="选择该分组可使用的 provider；选择顺序即组内 failover priority。"
        >
          <Select mode="multiple" options={providerOptions} placeholder="选择 Provider" />
        </Form.Item>
        <Form.Item label="状态" name="status">
          <Select options={[{ value: 'active', label: '启用' }, { value: 'disabled', label: '禁用' }]} />
        </Form.Item>
      </EntityFormModal>
    </PageContainer>
  );
}

export default Groups;
