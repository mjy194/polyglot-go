import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Form, Input, Button, Typography, App } from 'antd';
import { ThunderboltOutlined, UserOutlined, LockOutlined } from '@ant-design/icons';
import { useAuth } from '../context/AuthContext';

const { Title, Paragraph } = Typography;

function Login() {
  const { login, bootstrap } = useAuth();
  const navigate = useNavigate();
  const { message } = App.useApp();
  const [submitting, setSubmitting] = useState(false);
  const [mode, setMode] = useState<'login' | 'bootstrap'>('login');

  const onLogin = async (values: { email: string; password: string }) => {
    setSubmitting(true);
    try {
      await login(values.email.trim(), values.password);
      navigate('/', { replace: true });
    } catch (e: any) {
      message.error(e?.response?.data?.error || '登录失败,请检查用户名和密码');
    } finally {
      setSubmitting(false);
    }
  };

  const onBootstrap = async (values: {
    email: string;
    password: string;
    display_name?: string;
  }) => {
    setSubmitting(true);
    try {
      await bootstrap({ ...values, email: values.email.trim() });
      message.success('初始化完成,已登录');
      navigate('/', { replace: true });
    } catch (e: any) {
      const err = e?.response?.data?.error || '初始化失败';
      message.error(e?.response?.status === 409 ? '系统已初始化,请直接登录' : err);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={{ minHeight: '100vh', display: 'flex' }}>
      {/* 左:品牌区 */}
      <div
        style={{
          flex: 1,
          background: 'linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%)',
          color: '#fff',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          padding: '0 64px',
        }}
      >
        <div style={{ fontSize: 44, fontWeight: 700, marginBottom: 16 }}>🌐 Polyglot</div>
        <Title level={2} style={{ color: '#fff', marginTop: 0 }}>
          Universal AI API Gateway
        </Title>
        <Paragraph style={{ color: 'rgba(255,255,255,0.85)', fontSize: 16, maxWidth: 420 }}>
          统一接入 Anthropic / OpenAI / Gemini,账号池、用量审计,一处管理。
        </Paragraph>
      </div>

      {/* 右:登录表单 */}
      <div
        style={{
          width: 460,
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          padding: '0 56px',
          background: '#fff',
        }}
      >
        <Title level={3} style={{ marginBottom: 4 }}>
          <ThunderboltOutlined style={{ color: '#4f46e5' }} />{' '}
          {mode === 'login' ? '登录管理后台' : '首次初始化'}
        </Title>
        <Paragraph type="secondary">
          {mode === 'login' ? '使用用户名与密码登录' : '创建第一个管理员账号'}
        </Paragraph>

        {mode === 'login' ? (
          <Form
            layout="vertical"
            onFinish={onLogin}
            style={{ marginTop: 16 }}
            requiredMark={false}
          >
            <Form.Item label="用户名" name="email" rules={[{ required: true, message: '请输入用户名' }]}>
              <Input
                prefix={<UserOutlined />}
                placeholder="admin@example.com"
                size="large"
                autoFocus
              />
            </Form.Item>
            <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
              <Input.Password prefix={<LockOutlined />} placeholder="密码" size="large" />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" size="large" block loading={submitting}>
                登录
              </Button>
            </Form.Item>
            <Button type="link" block onClick={() => setMode('bootstrap')}>
              首次使用?初始化管理员
            </Button>
          </Form>
        ) : (
          <Form
            layout="vertical"
            onFinish={onBootstrap}
            style={{ marginTop: 16 }}
            requiredMark={false}
          >
            <Form.Item label="用户名" name="email" rules={[{ required: true, message: '请输入用户名' }]}>
              <Input prefix={<UserOutlined />} placeholder="admin@example.com" size="large" autoFocus />
            </Form.Item>
            <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
              <Input.Password prefix={<LockOutlined />} placeholder="设置密码" size="large" />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" size="large" block loading={submitting}>
                初始化并登录
              </Button>
            </Form.Item>
            <Button type="link" block onClick={() => setMode('login')}>
              已有账号?返回登录
            </Button>
          </Form>
        )}
      </div>
    </div>
  );
}

export default Login;
