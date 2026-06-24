import { useState } from 'react';
import { Form, Input, Select, Tag, Modal, Space, App, Popconfirm, Switch, Alert } from 'antd';
import {
  listProxies,
  upsertProxy,
  deleteProxy,
  testProxy,
} from '../api/client';
import { useCrud } from '../hooks/useCrud';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import EntityFormModal from '../components/EntityFormModal';
import type { Proxy } from '../api/types';

// 从 URL scheme 推导代理类型（http/https/socks5）。
function schemeOf(url: string): string {
  try {
    return new URL(url).protocol.replace(':', '');
  } catch {
    return '?';
  }
}

type TestResult = {
  success: boolean;
  proxy?: string;
  target?: string;
  status?: number;
  latency_ms?: number;
  exit_ip?: string;
  error?: string;
};

function Proxies() {
  const { message } = App.useApp();
  const proxyCrud = useCrud<Proxy>({
    list: listProxies,
    save: upsertProxy,
    deps: [],
    defaults: { status: 'active' } as Partial<Proxy>,
  });

  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<TestResult | null>(null);

  const toggleStatus = async (proxy: Proxy, active: boolean) => {
    try {
      await upsertProxy({ ...proxy, status: active ? 'active' : 'disabled' });
      message.success(active ? '已启用' : '已禁用');
      proxyCrud.reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '更新失败');
    }
  };

  const runTest = async (proxy: Proxy) => {
    setTesting(true);
    setTestResult(null);
    try {
      const res = await testProxy(proxy.id);
      setTestResult({ ...res, proxy: proxy.name });
    } catch (e: any) {
      setTestResult({ success: false, proxy: proxy.name, error: e?.response?.data?.error || e?.message });
    } finally {
      setTesting(false);
    }
  };

  const removeProxy = async (id: string) => {
    try {
      await deleteProxy(id);
      message.success('已删除');
      proxyCrud.reload();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '删除失败');
    }
  };

  return (
    <PageContainer description="管理出站网络代理（给 Provider 关联代理请到 Providers 页编辑）">
      <CrudTable<Proxy>
        data={proxyCrud.data}
        loading={proxyCrud.loading}
        onReload={proxyCrud.reload}
        onCreate={proxyCrud.openCreate}
        createText="新增代理"
        columns={[
          { title: '名称', dataIndex: 'name' },
          { title: '类型', width: 90, render: (_: unknown, r: Proxy) => <Tag>{schemeOf(r.url)}</Tag> },
          { title: 'URL', dataIndex: 'url', ellipsis: true },
          {
            title: '状态',
            dataIndex: 'status',
            width: 80,
            render: (s: string, r: Proxy) => (
              <Switch size="small" checked={s === 'active'} onChange={(v) => toggleStatus(r, v)} />
            ),
          },
          {
            title: '操作',
            fixed: 'right',
            width: 170,
            render: (_: unknown, r: Proxy) => (
              <Space>
                <a onClick={() => proxyCrud.openEdit(r)}>编辑</a>
                <a onClick={() => runTest(r)}>测试</a>
                <Popconfirm title="删除该代理？" onConfirm={() => removeProxy(r.id)}>
                  <a style={{ color: '#ff4d4f' }}>删除</a>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <EntityFormModal
        title={proxyCrud.isEditing ? '编辑代理' : '新增代理'}
        open={proxyCrud.open}
        form={proxyCrud.form}
        saving={proxyCrud.saving}
        onOk={proxyCrud.submit}
        onCancel={proxyCrud.closeModal}
      >
        <Form.Item name="id" hidden>
          <Input />
        </Form.Item>
        <Form.Item label="名称" name="name" rules={[{ required: true }]}>
          <Input placeholder="primary-proxy" />
        </Form.Item>
        <Form.Item
          label="URL"
          name="url"
          rules={[{ required: true }]}
          extra="scheme://host:port，scheme 决定类型（http / https / socks5）"
        >
          <Input placeholder="http://127.0.0.1:8888" />
        </Form.Item>
        <Form.Item label="用户名" name="username">
          <Input placeholder="（可选）代理认证用户名" />
        </Form.Item>
        <Form.Item label="密码" name="password">
          <Input.Password placeholder="（可选）代理认证密码" />
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

      <Modal
        title="代理测试结果"
        open={!!testResult || testing}
        footer={null}
        onCancel={() => setTestResult(null)}
      >
        {testing ? (
          <div>正在通过代理发起测试请求…</div>
        ) : testResult ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Alert
              type={testResult.success ? 'success' : 'error'}
              showIcon
              message={testResult.success ? '代理可用' : '代理不可用'}
              description={
                testResult.error ? (
                  <span style={{ wordBreak: 'break-all' }}>{testResult.error}</span>
                ) : undefined
              }
            />
            <Descriptions kvs={[
              ['代理', testResult.proxy ?? '—'],
              ['目标', testResult.target ?? '—'],
              ['HTTP 状态', testResult.status != null ? String(testResult.status) : '—'],
              ['延迟', testResult.latency_ms != null ? `${testResult.latency_ms} ms` : '—'],
              ['出口 IP', testResult.exit_ip || '—'],
            ]} />
          </Space>
        ) : null}
      </Modal>
    </PageContainer>
  );
}

function Descriptions({ kvs }: { kvs: [string, string][] }) {
  return (
    <div style={{ fontSize: 13 }}>
      {kvs.map(([k, v]) => (
        <div key={k} style={{ display: 'flex', justifyContent: 'space-between', padding: '2px 0' }}>
          <span style={{ color: '#888' }}>{k}</span>
          <span style={{ marginLeft: 16, wordBreak: 'break-all', textAlign: 'right' }}>{v}</span>
        </div>
      ))}
    </div>
  );
}

export default Proxies;
