import { Routes, Route, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { ReactNode } from 'react';
import { Result, Button, Spin } from 'antd';
import Layout from './components/Layout';
import ErrorBoundary from './components/ErrorBoundary';
import { useAuth } from './context/AuthContext';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Providers from './pages/Providers';
import Proxies from './pages/Proxies';
import Groups from './pages/Groups';
import ApiKeys from './pages/ApiKeys';
import Users from './pages/Users';
import Roles from './pages/Roles';
import Adapters from './pages/Adapters';
import Logs from './pages/Logs';
import Usage from './pages/Usage';
import Settings from './pages/Settings';

function NotFound() {
  const navigate = useNavigate();
  return (
    <Result
      status="404"
      title="404"
      subTitle="页面不存在"
      extra={
        <Button type="primary" onClick={() => navigate('/')}>
          返回首页
        </Button>
      }
    />
  );
}

// 登录页公开;其余路由需登录。会话恢复未完成前显示 loading。
function Shell({ children }: { children: ReactNode }) {
  const location = useLocation();
  const { isAuthenticated, ready } = useAuth();

  if (location.pathname === '/login') return <>{children}</>;

  if (!ready) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return (
    <Layout>
      <ErrorBoundary>{children}</ErrorBoundary>
    </Layout>
  );
}

function App() {
  return (
    <Shell>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/" element={<Dashboard />} />
        <Route path="/providers" element={<Providers />} />
        <Route path="/proxies" element={<Proxies />} />
        <Route path="/groups" element={<Groups />} />
        <Route path="/api-keys" element={<ApiKeys />} />
        <Route path="/users" element={<Users />} />
        <Route path="/roles" element={<Roles />} />
        <Route path="/adapters" element={<Adapters />} />
        <Route path="/logs" element={<Logs />} />
        <Route path="/usage" element={<Usage />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </Shell>
  );
}

export default App;
