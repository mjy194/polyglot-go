import { useEffect, useRef, useState } from 'react';
import { App, Form, Input, Popconfirm, Select, Space, Switch, Tag, Tooltip } from 'antd';
import {
  deleteProvider,
  fetchProviderHealthHourly,
  listGroups,
  listProviderModelMappings,
  listProviderGroups,
  listProviderProxies,
  listProviders,
  listProxies,
  setProviderGroups,
  setProviderProxies,
  upsertProvider,
  upsertProviderModelMapping,
  type HealthBucket,
} from '../api/client';
import type { Provider } from '../api/types';
import { useCrud } from '../hooks/useCrud';
import { useFetch } from '../hooks/useFetch';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import EntityFormModal from '../components/EntityFormModal';
import ModelMappingsModal from './provider/ModelMappingsModal';

const MODE_OPTIONS = [
  { value: 'passthrough', label: 'passthrough（直连上游）' },
  { value: 'adapter', label: 'adapter（路由到指定 adapter）' },
];

const STRATEGIES = [
  { value: 'failover', label: 'failover（主备）' },
  { value: 'round_robin', label: 'round_robin（轮询）' },
  { value: 'random', label: 'random（随机）' },
];

function Providers() {
  const { message } = App.useApp();
  const { data: allProxies } = useFetch(listProxies, []);
  const { data: allGroups } = useFetch(listGroups, []);
  const proxyOptions = (allProxies || []).map((p) => ({ value: p.id, label: p.name }));
  const groupOptions = (allGroups || []).map((g) => ({ value: g.id, label: g.name }));

  const toggleStatus = async (provider: Provider, active: boolean) => {
    try {
      await upsertProvider({ ...provider, status: active ? 'active' : 'disabled' });
      message.success(active ? '已启用' : '已禁用');
      crud.reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '更新失败');
    }
  };

  const remove = async (provider: Provider) => {
    try {
      await deleteProvider(provider.id);
      message.success('已删除');
      crud.reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '删除失败');
    }
  };

  // 复制时暂存源的模型映射，保存新 provider 后一并复制过去。
  const copyMappingsRef = useRef<{ from_model: string; to_model: string }[] | null>(null);

  const crud = useCrud<Provider>({
    list: listProviders,
    save: async (values: any) => {
      const saved = await upsertProvider(values);
      // 关联代理：选择顺序即 priority（failover 排序用）。
      const ids: string[] = values.proxy_ids || [];
      await setProviderProxies(
        saved.id,
        ids.map((id, i) => ({ proxy_id: id, priority: i })),
      );
      const groupIds: string[] = values.group_ids || [];
      await setProviderGroups(
        saved.id,
        groupIds.map((id, i) => ({ group_id: id, priority: i })),
      );
      // 复制时带上源 provider 的模型映射。
      if (copyMappingsRef.current) {
        await Promise.all(
          copyMappingsRef.current.map((m) =>
            upsertProviderModelMapping(saved.id, { from_model: m.from_model, to_model: m.to_model }),
          ),
        );
        copyMappingsRef.current = null;
      }
      return saved;
    },
    deps: [],
    defaults: { status: 'active', mode: 'passthrough', proxy_strategy: 'failover' } as Partial<Provider>,
  });

  // 编辑时把已关联的代理 seed 进表单。
  const openEdit = async (row: Provider) => {
    copyMappingsRef.current = null; // 编辑非复制，清掉复制暂存
    await crud.openEdit(row);
    try {
      const [proxyLinks, groupLinks] = await Promise.all([
        listProviderProxies(row.id), // 已按 priority ASC
        listProviderGroups(row.id),
      ]);
      crud.form.setFieldsValue({
        proxy_ids: proxyLinks.map((l) => l.proxy_id),
        group_ids: groupLinks.map((l) => l.group_id),
      });
    } catch {
      /* ignore */
    }
  };

  // 复制：以源 provider 字段预填新建表单（id 清空 → 新建），代理关联一并带上。
  const openCopy = async (row: Provider) => {
    crud.openCreate(); // 进入新建态（isEditing=false），清空 + defaults
    crud.form.setFieldsValue({
      name: `${row.name}-copy`,
      type: row.type,
      base_url: row.base_url,
      mode: row.mode || 'passthrough',
      adapter: row.adapter || '',
      api_key: row.api_key || '',
      default_headers: row.default_headers || '',
      proxy_strategy: row.proxy_strategy || 'failover',
      status: row.status || 'active',
    });
    try {
      const [proxyLinks, groupLinks, mappings] = await Promise.all([
        listProviderProxies(row.id),
        listProviderGroups(row.id),
        listProviderModelMappings(row.id),
      ]);
      crud.form.setFieldsValue({
        proxy_ids: proxyLinks.map((l) => l.proxy_id),
        group_ids: groupLinks.map((l) => l.group_id),
      });
      // 暂存映射，保存时复制到新 provider。
      copyMappingsRef.current = mappings.map((m) => ({ from_model: m.from_model, to_model: m.to_model }));
    } catch {
      /* ignore */
    }
  };

  // 普通新建：清掉复制暂存，避免上一轮复制的映射泄漏。
  const openCreate = () => {
    copyMappingsRef.current = null;
    crud.openCreate();
    const defaultGroup = allGroups?.find((g) => g.name === 'default');
    if (defaultGroup) {
      crud.form.setFieldsValue({ group_ids: [defaultGroup.id] });
    }
  };

  // 列表里显示每个 provider 已分配的分组/代理名。
  const [groupNames, setGroupNames] = useState<Record<string, string[]>>({});
  const [proxyNames, setProxyNames] = useState<Record<string, string[]>>({});
  const [healthHourly, setHealthHourly] = useState<Record<string, HealthBucket[]>>({});
  useEffect(() => {
    const data = crud.data;
    if (!data) return;
    let cancelled = false;
    (async () => {
      const [hourly, proxyEntries, groupEntries] = await Promise.all([
        fetchProviderHealthHourly().catch(() => ({})),
        Promise.all(
          data.map(async (p) => {
            const links = await listProviderProxies(p.id).catch(() => []);
            return [p.id, links.map((l) => l.name).filter(Boolean)] as [string, string[]];
          }),
        ),
        Promise.all(
          data.map(async (p) => {
            const links = await listProviderGroups(p.id).catch(() => []);
            return [p.id, links.map((l) => l.name).filter(Boolean)] as [string, string[]];
          }),
        ),
      ]);
      if (!cancelled) {
        setHealthHourly(hourly as Record<string, HealthBucket[]>);
        setProxyNames(Object.fromEntries(proxyEntries));
        setGroupNames(Object.fromEntries(groupEntries));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [crud.data]);

  const [mmFor, setMmFor] = useState<Provider | null>(null);

  return (
    <PageContainer description="管理上游 AI 服务提供方（模式 + 代理关联 + 模型映射）">
      <CrudTable<Provider>
        data={crud.data}
        loading={crud.loading}
        onReload={crud.reload}
        onCreate={openCreate}
        createText="新增 Provider"
        columns={[
          { title: '名称', dataIndex: 'name', sorter: (a, b) => a.name.localeCompare(b.name) },
          { title: '类型', dataIndex: 'type' },
          { title: 'Base URL', dataIndex: 'base_url', ellipsis: true },
          {
            title: '模式',
            dataIndex: 'mode',
            width: 120,
            render: (m: string) => {
              const mode = m || 'passthrough';
              return <Tag color={mode === 'adapter' ? 'blue' : 'green'}>{mode}</Tag>;
            },
          },
          {
            title: '健康度',
            dataIndex: 'name',
            width: 200,
            render: (name: string) => {
              const buckets = healthHourly[name];
              if (!buckets || buckets.every((b) => !b || b.total === 0)) {
                return <Tag>—</Tag>;
              }
              const filled = buckets.filter((b) => b && b.total > 0);
              const max = Math.max(1, ...filled.map((b) => b.total));
              return (
                <Space size={2} style={{ height: 22, alignItems: 'flex-end' }}>
                  {buckets.map((b, i) => {
                    const total = b?.total ?? 0;
                    const rate = total ? (b.successes ?? 0) / total : null;
                    const color =
                      rate == null ? '#e8e8e8' : rate >= 0.95 ? '#52c41a' : rate >= 0.8 ? '#faad14' : '#ff4d4f';
                    const h = total ? 5 + 16 * (total / max) : 3;
                    const hoursAgo = 23 - i;
                    return (
                      <Tooltip
                        key={i}
                        title={
                          total
                            ? `${hoursAgo}h 前：${total} 次 · 成功 ${Math.round((rate ?? 0) * 100)}%`
                            : `${hoursAgo}h 前：无请求`
                        }
                      >
                        <div
                          style={{
                            width: 6,
                            height: h,
                            background: color,
                            borderRadius: 1,
                          }}
                        />
                      </Tooltip>
                    );
                  })}
                </Space>
              );
            },
          },
          {
            title: '分组',
            dataIndex: 'id',
            render: (id: string) => {
              const names = groupNames[id] || [];
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
            title: '代理',
            dataIndex: 'id',
            render: (id: string) => {
              const names = proxyNames[id] || [];
              if (!names.length) return <Tag>—</Tag>;
              return (
                <Space size={4} wrap>
                  {names.map((n) => (
                    <Tag key={n} color="purple">
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
            render: (s: string, r: Provider) => (
              <Switch size="small" checked={s === 'active'} onChange={(v) => toggleStatus(r, v)} />
            ),
          },
          {
            title: '操作',
            fixed: 'right',
            width: 230,
            render: (_: unknown, r: Provider) => (
              <Space>
                <a onClick={() => openEdit(r)}>编辑</a>
                <a onClick={() => openCopy(r)}>复制</a>
                <a onClick={() => setMmFor(r)}>模型映射</a>
                <Popconfirm
                  title={`删除 provider「${r.name}」？`}
                  description="连同其代理关联与模型映射一起删除。"
                  onConfirm={() => remove(r)}
                  okText="删除"
                  okButtonProps={{ danger: true }}
                >
                  <a style={{ color: '#ff4d4f' }}>删除</a>
                </Popconfirm>
              </Space>
            ),
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
            showSearch
            options={[
              { value: 'anthropic', label: 'anthropic' },
              { value: 'openai', label: 'openai' },
              { value: 'gemini', label: 'gemini' },
            ]}
          />
        </Form.Item>
        <Form.Item label="Base URL" name="base_url">
          <Input placeholder="https://..." />
        </Form.Item>
        <Form.Item
          label="路由模式"
          name="mode"
          extra="passthrough（默认）：本 provider 直连上游（Base URL + API Key + 关联代理）。adapter：路由到指定 adapter。"
        >
          <Select options={MODE_OPTIONS} />
        </Form.Item>
        <Form.Item
          noStyle
          shouldUpdate={(prev, cur) => prev.mode !== cur.mode}
        >
          {({ getFieldValue }) =>
            getFieldValue('mode') === 'adapter' ? (
              <Form.Item
                label="Adapter"
                name="adapter"
                rules={[{ required: true, message: 'adapter 模式需指定 adapter 名称' }]}
                extra="已注册 adapter 的 provider 名（如 uipath）。"
              >
                <Input placeholder="uipath" />
              </Form.Item>
            ) : null
          }
        </Form.Item>
        <Form.Item
          label="API Key"
          name="api_key"
          extra="passthrough 模式下用于上游鉴权（adapter 模式由账号池管理，可不填）"
        >
          <Input.Password placeholder="上游 API key" />
        </Form.Item>
        <Form.Item label="默认 Headers (JSON)" name="default_headers">
          <Input.TextArea rows={2} placeholder='{"x-foo":"bar"}' />
        </Form.Item>
        <Form.Item
          label="服务分组"
          name="group_ids"
          extra="选择该 provider 服务的分组；选择顺序即组内 failover priority。"
        >
          <Select mode="multiple" options={groupOptions} placeholder="选择分组" />
        </Form.Item>
        <Form.Item
          label="出站代理"
          name="proxy_ids"
          extra="多选关联的代理；选择顺序即 failover priority（越靠前越优先）。代理在「代理」页管理。"
        >
          <Select mode="multiple" options={proxyOptions} placeholder="（可选）选择代理" />
        </Form.Item>
        <Form.Item label="代理选择策略" name="proxy_strategy">
          <Select options={STRATEGIES} />
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

      {mmFor && <ModelMappingsModal provider={mmFor} onClose={() => setMmFor(null)} />}
    </PageContainer>
  );
}

export default Providers;
