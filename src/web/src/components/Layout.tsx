import { ReactNode, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { Layout as AntLayout, Menu, Button, Dropdown, Avatar, Space, Typography } from 'antd';
import {
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  UserOutlined,
  LogoutOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { navGroups } from '../router';
import { useAuth } from '../context/AuthContext';

const { Header, Sider, Content } = AntLayout;
const { Text } = Typography;

const COLLAPSE_KEY = 'polyglot_sider_collapsed';

// antd Menu items:展开时分组显示,折叠时扁平(分组态在 Sider 折叠下不会塌缩成图标)
const groupedItems = navGroups.map((g) => ({
  key: g.key,
  type: 'group' as const,
  label: g.title,
  children: g.items.map((it) => ({
    key: it.path,
    icon: it.icon,
    label: <Link to={it.path}>{it.label}</Link>,
  })),
}));

const flatItems = navGroups.flatMap((g) =>
  g.items.map((it) => ({
    key: it.path,
    icon: it.icon,
    label: <Link to={it.path}>{it.label}</Link>,
  })),
);

function Layout({ children }: { children: ReactNode }) {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, logout } = useAuth();
  const [collapsed, setCollapsed] = useState(
    () => localStorage.getItem(COLLAPSE_KEY) === '1',
  );

  const toggle = () => {
    const next = !collapsed;
    setCollapsed(next);
    localStorage.setItem(COLLAPSE_KEY, next ? '1' : '0');
  };

  const onLogout = () => {
    logout();
    navigate('/login', { replace: true });
  };

  const userMenu = {
    items: [
      {
        key: 'who',
        label: <Text strong>{user?.email || '未登录'}</Text>,
        disabled: true,
      },
      { type: 'divider' as const },
      { key: 'settings', icon: <SettingOutlined />, label: '设置' },
      { key: 'logout', icon: <LogoutOutlined />, label: '登出', danger: true },
    ],
    onClick: ({ key }: { key: string }) => {
      if (key === 'logout') onLogout();
      if (key === 'settings') navigate('/settings');
    },
  };

  return (
    <AntLayout style={{ minHeight: '100vh' }}>
      <Sider
        collapsible
        collapsed={collapsed}
        trigger={null}
        breakpoint="lg"
        onBreakpoint={(broken) => setCollapsed(broken)}
        width={220}
        collapsedWidth={72}
      >
        <div
          style={{
            height: 56,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#fff',
            fontSize: collapsed ? 22 : 18,
            fontWeight: 600,
            letterSpacing: 0.3,
          }}
        >
          {collapsed ? '🌐' : '🌐 Polyglot'}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          items={collapsed ? flatItems : groupedItems}
          style={{ borderInlineEnd: 'none' }}
        />
      </Sider>

      <AntLayout>
        <Header
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            borderBottom: '1px solid #eef0f3',
            position: 'sticky',
            top: 0,
            zIndex: 10,
          }}
        >
          <Button
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={toggle}
            style={{ fontSize: 16 }}
          />
          <Space size={16}>
            <Dropdown menu={userMenu} placement="bottomRight">
              <Avatar
                size="small"
                icon={<UserOutlined />}
                style={{ cursor: 'pointer', backgroundColor: '#4f46e5' }}
              />
            </Dropdown>
          </Space>
        </Header>

        <Content style={{ margin: 24 }}>{children}</Content>
      </AntLayout>
    </AntLayout>
  );
}

export default Layout;
