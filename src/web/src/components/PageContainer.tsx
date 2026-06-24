import { ReactNode } from 'react';
import { Breadcrumb, Typography, Space } from 'antd';
import { Link, useLocation } from 'react-router-dom';
import { labelForPath } from '../router';

const { Title, Text } = Typography;

interface Props {
  title?: string;
  description?: ReactNode;
  extra?: ReactNode;
  children: ReactNode;
}

// 统一页面外壳:面包屑 + 标题区 + 内容
function PageContainer({ title, description, extra, children }: Props) {
  const location = useLocation();
  const current = title ?? labelForPath(location.pathname);

  return (
    <div>
      <Breadcrumb
        style={{ marginBottom: 12 }}
        items={[{ title: <Link to="/">首页</Link> }, { title: current }]}
      />
      <div
        style={{
          display: 'flex',
          alignItems: 'flex-start',
          justifyContent: 'space-between',
          marginBottom: 16,
          gap: 16,
        }}
      >
        <Space direction="vertical" size={2}>
          <Title level={3} style={{ margin: 0 }}>
            {current}
          </Title>
          {description && <Text type="secondary">{description}</Text>}
        </Space>
        {extra && <div>{extra}</div>}
      </div>
      {children}
    </div>
  );
}

export default PageContainer;
