import { useEffect, useState } from 'react';
import { Row, Col, Card, Statistic, Table, Tag, Skeleton, Empty, Typography } from 'antd';
import {
  ThunderboltOutlined,
  CheckCircleOutlined,
  DashboardOutlined,
  ClusterOutlined,
} from '@ant-design/icons';
import { getStats, listAdapters, listRequestLogs } from '../api/client';
import PageContainer from '../components/PageContainer';
import StatusBadge from '../components/StatusBadge';
import type { Stats, Adapter, RequestLog } from '../api/types';

const { Text } = Typography;

function AdapterCloud({ adapters }: { adapters: Adapter[] }) {
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12 }}>
      {adapters.map((a) => (
        <Card key={a.id} size="small" style={{ minWidth: 180 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Text strong>{a.name}</Text>
            <StatusBadge status={a.status} />
          </div>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {a.type}
          </Text>
        </Card>
      ))}
    </div>
  );
}

function Dashboard() {
  const [stats, setStats] = useState<Stats>();
  const [adapters, setAdapters] = useState<Adapter[]>([]);
  const [logs, setLogs] = useState<RequestLog[]>([]);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let stop = false;
    const load = async () => {
      try {
        const [s, a, l] = await Promise.all([
          getStats(),
          listAdapters(),
          listRequestLogs({ limit: 8 }),
        ]);
        if (stop) return;
        setStats(s);
        setAdapters(a || []);
        setLogs(l || []);
      } catch {
        /* 401 等已由 axios 拦截器统一处理 */
      } finally {
        if (!stop) setReady(true);
      }
    };
    load();
    const timer = setInterval(load, 5000);
    return () => {
      stop = true;
      clearInterval(timer);
    };
  }, []);

  return (
    <PageContainer description="网关运行概览,每 5 秒自动刷新">
      <Skeleton loading={!ready} active paragraph={{ rows: 6 }}>
        <Row gutter={16}>
          <Col xs={24} sm={8}>
            <Card>
              <Statistic
                title="总请求数"
                value={stats?.requests_total ?? 0}
                prefix={<ThunderboltOutlined style={{ color: '#4f46e5' }} />}
              />
            </Card>
          </Col>
          <Col xs={24} sm={8}>
            <Card>
              <Statistic
                title="成功率"
                value={((stats?.success_rate ?? 0) * 100).toFixed(1)}
                suffix="%"
                valueStyle={{ color: '#16a34a' }}
                prefix={<CheckCircleOutlined />}
              />
            </Card>
          </Col>
          <Col xs={24} sm={8}>
            <Card>
              <Statistic
                title="平均延迟"
                value={stats?.average_latency_ms ?? 0}
                suffix="ms"
                prefix={<DashboardOutlined style={{ color: '#d97706' }} />}
              />
            </Card>
          </Col>
        </Row>

        <Card
          title={
            <span>
              <ClusterOutlined /> Adapters
            </span>
          }
          style={{ marginTop: 16 }}
        >
          {adapters.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无 adapter" />
          ) : (
            <AdapterCloud adapters={adapters} />
          )}
        </Card>

        <Card title="最近请求" style={{ marginTop: 16 }}>
          <Table<RequestLog>
            rowKey="id"
            size="small"
            pagination={false}
            dataSource={logs}
            locale={{
              emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无请求" />,
            }}
            columns={[
              { title: '时间', dataIndex: 'created_at' },
              { title: '协议', dataIndex: 'protocol' },
              { title: '模型', dataIndex: 'model' },
              {
                title: '状态',
                dataIndex: 'status_code',
                render: (code: number, r) => (
                  <Tag color={r.success ? 'success' : 'error'}>{code}</Tag>
                ),
              },
              { title: '延迟(ms)', dataIndex: 'latency_ms' },
            ]}
          />
        </Card>
      </Skeleton>
    </PageContainer>
  );
}

export default Dashboard;
