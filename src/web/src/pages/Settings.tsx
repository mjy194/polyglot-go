import { Card, Button, Descriptions } from 'antd';
import { useAuth } from '../context/AuthContext';
import PageContainer from '../components/PageContainer';

function Settings() {
  const { user, logout } = useAuth();

  return (
    <PageContainer description="当前后台账号">
      <Card title="当前账号" style={{ marginBottom: 16 }}>
        <Descriptions column={1} size="small">
          <Descriptions.Item label="邮箱">{user?.email || '—'}</Descriptions.Item>
          <Descriptions.Item label="昵称">{user?.display_name || '—'}</Descriptions.Item>
          <Descriptions.Item label="状态">{user?.status || '—'}</Descriptions.Item>
        </Descriptions>
        <Button danger style={{ marginTop: 12 }} onClick={logout}>
          登出
        </Button>
      </Card>
    </PageContainer>
  );
}

export default Settings;
