import { useEffect, useState } from 'react';
import { Table, Empty } from 'antd';
import { listAdapters, listAdapterInstances } from '../api/client';
import { useFetch } from '../hooks/useFetch';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import StatusBadge from '../components/StatusBadge';
import { formatTime } from '../utils/format';
import type { Adapter, AdapterInstance } from '../api/types';

function InstanceTable({ adapterId }: { adapterId: string }) {
  const [rows, setRows] = useState<AdapterInstance[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let stop = false;
    listAdapterInstances(adapterId)
      .then((r) => !stop && setRows(r || []))
      .catch(() => !stop && setRows([]))
      .finally(() => !stop && setLoading(false));
    return () => {
      stop = true;
    };
  }, [adapterId]);

  return (
    <Table<AdapterInstance>
      rowKey="ID"
      size="small"
      loading={loading}
      pagination={false}
      dataSource={rows}
      locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无实例" /> }}
      columns={[
        { title: 'Instance ID', dataIndex: 'ID' },
        { title: 'Provider', dataIndex: 'Provider' },
        { title: 'Callback', dataIndex: 'CallbackAddr' },
        { title: 'Capabilities', dataIndex: 'Capabilities' },
        { title: '状态', dataIndex: 'Status', render: (s: string) => <StatusBadge status={s} /> },
        { title: '心跳', dataIndex: 'LastHeartbeatAt', render: (v?: string) => v || '—' },
      ]}
    />
  );
}

function Adapters() {
  const { data, loading, reload } = useFetch<Adapter[]>(() => listAdapters(), []);

  return (
    <PageContainer description="已注册的后端适配器及其运行实例">
      <CrudTable<Adapter>
        data={data}
        loading={loading}
        onReload={reload}
        expandable={{ expandedRowRender: (r) => <InstanceTable adapterId={r.id} /> }}
        columns={[
          { title: '名称', dataIndex: 'name', sorter: (a, b) => a.name.localeCompare(b.name) },
          { title: '类型', dataIndex: 'type' },
          { title: '状态', dataIndex: 'status', render: (s: string) => <StatusBadge status={s} /> },
          { title: '创建时间', dataIndex: 'created_at', render: (v: string) => formatTime(v) },
        ]}
      />
    </PageContainer>
  );
}

export default Adapters;
