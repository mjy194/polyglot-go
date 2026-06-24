import { ReactNode } from 'react';
import {
  DashboardOutlined,
  ApiOutlined,
  SwapOutlined,
  KeyOutlined,
  TeamOutlined,
  SafetyOutlined,
  ClusterOutlined,
  FileTextOutlined,
  BarChartOutlined,
  SettingOutlined,
} from '@ant-design/icons';

export interface NavItem {
  path: string;
  label: string;
  icon: ReactNode;
}

export interface NavGroup {
  key: string;
  title: string;
  items: NavItem[];
}

// 侧栏菜单 + 面包屑 + 页标题的单一数据源
export const navGroups: NavGroup[] = [
  {
    key: 'overview',
    title: '概览',
    items: [{ path: '/', label: 'Dashboard', icon: <DashboardOutlined /> }],
  },
  {
    key: 'config',
    title: '配置',
    items: [
      { path: '/providers', label: 'Providers', icon: <ApiOutlined /> },
      { path: '/model-mappings', label: '模型映射', icon: <SwapOutlined /> },
      { path: '/adapters', label: 'Adapters', icon: <ClusterOutlined /> },
    ],
  },
  {
    key: 'access',
    title: '访问控制',
    items: [
      { path: '/users', label: '用户', icon: <TeamOutlined /> },
      { path: '/roles', label: '角色', icon: <SafetyOutlined /> },
      { path: '/api-keys', label: 'API Keys', icon: <KeyOutlined /> },
    ],
  },
  {
    key: 'audit',
    title: '审计',
    items: [
      { path: '/logs', label: '请求日志', icon: <FileTextOutlined /> },
      { path: '/usage', label: '用量', icon: <BarChartOutlined /> },
    ],
  },
  {
    key: 'system',
    title: '系统',
    items: [{ path: '/settings', label: '设置', icon: <SettingOutlined /> }],
  },
];

const allItems = navGroups.flatMap((g) => g.items);

export function labelForPath(path: string): string {
  return allItems.find((i) => i.path === path)?.label ?? '';
}
