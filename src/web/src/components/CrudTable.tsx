import { ReactNode } from 'react';
import { Card, Table, Button, Empty, Space } from 'antd';
import { ReloadOutlined, PlusOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

interface Props<T> {
  columns: ColumnsType<T>;
  data: T[] | undefined;
  loading?: boolean;
  rowKey?: string;
  onReload?: () => void;
  onCreate?: () => void;
  createText?: string;
  emptyText?: string;
  extra?: ReactNode;
  expandable?: React.ComponentProps<typeof Table<T>>['expandable'];
}

// 统一列表卡片:右上角刷新/新建,内置分页 + 空态
function CrudTable<T extends object>({
  columns,
  data,
  loading,
  rowKey = 'id',
  onReload,
  onCreate,
  createText = '新建',
  emptyText = '暂无数据',
  extra,
  expandable,
}: Props<T>) {
  return (
    <Card
      styles={{ body: { padding: 0 } }}
      style={{ overflow: 'hidden' }}
      title={null}
      extra={
        <Space style={{ padding: '12px 16px' }}>
          {extra}
          {onReload && (
            <Button icon={<ReloadOutlined />} onClick={onReload}>
              刷新
            </Button>
          )}
          {onCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
              {createText}
            </Button>
          )}
        </Space>
      }
    >
      <Table<T>
        rowKey={rowKey}
        columns={columns}
        dataSource={data || []}
        loading={loading}
        size="middle"
        scroll={{ x: 'max-content' }}
        expandable={expandable}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText} /> }}
        pagination={{
          pageSize: 10,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
        }}
      />
    </Card>
  );
}

export default CrudTable;
