import { theme } from 'antd';
import type { ThemeConfig } from 'antd';

// 统一设计 token —— 现代 SaaS 控制台风格(indigo 主色 + 中性灰 + 弱阴影)。
// 一处定义,全局生效,页面/组件不再写零散 style。
export type AppThemeMode = 'light' | 'dark';

export function createAppTheme(mode: AppThemeMode): ThemeConfig {
  const dark = mode === 'dark';

  return {
    algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorPrimary: '#4f46e5',
      colorInfo: '#4f46e5',
      colorSuccess: '#16a34a',
      colorWarning: '#d97706',
      colorError: '#dc2626',
      colorBgLayout: dark ? '#0b1120' : '#f7f8fa',
      colorBgContainer: dark ? '#111827' : '#ffffff',
      colorBgElevated: dark ? '#111827' : '#ffffff',
      colorBorder: dark ? '#253041' : '#d9dce3',
      colorBorderSecondary: dark ? '#1f2937' : '#eef0f3',
      colorText: dark ? '#e5e7eb' : '#101828',
      colorTextSecondary: dark ? '#94a3b8' : '#667085',
      borderRadius: 8,
      fontSize: 14,
      wireframe: false,
      fontFamily:
        "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
    },
    components: {
      Layout: {
        headerBg: dark ? '#111827' : '#ffffff',
        headerHeight: 56,
        headerPadding: '0 20px',
        bodyBg: dark ? '#0b1120' : '#f7f8fa',
        siderBg: dark ? '#0f172a' : '#ffffff',
      },
      Menu: {
        itemBg: dark ? '#0f172a' : '#ffffff',
        itemColor: dark ? 'rgba(255,255,255,0.72)' : '#344054',
        itemHoverBg: dark ? 'rgba(255,255,255,0.06)' : '#f2f4f7',
        itemHoverColor: dark ? '#ffffff' : '#101828',
        itemSelectedBg: dark ? '#4f46e5' : '#eef2ff',
        itemSelectedColor: dark ? '#ffffff' : '#4338ca',
        groupTitleColor: dark ? 'rgba(255,255,255,0.45)' : '#667085',
        darkItemBg: '#0f172a',
        darkSubMenuItemBg: '#0f172a',
        darkItemSelectedBg: '#4f46e5',
        darkItemHoverBg: 'rgba(255,255,255,0.06)',
        darkItemColor: 'rgba(255,255,255,0.72)',
        darkItemSelectedColor: '#ffffff',
        itemBorderRadius: 8,
        itemMarginInline: 8,
      },
      Card: {
        borderRadiusLG: 12,
        boxShadowTertiary: dark
          ? '0 1px 2px rgba(0,0,0,0.28), 0 1px 3px rgba(0,0,0,0.18)'
          : '0 1px 2px rgba(16,24,40,0.06), 0 1px 3px rgba(16,24,40,0.04)',
      },
      Table: {
        headerBg: dark ? '#0f172a' : '#fafafa',
        headerColor: dark ? '#cbd5e1' : '#475467',
        rowHoverBg: dark ? '#1e293b' : 'rgba(79,70,229,0.08)',
        cellPaddingBlock: 12,
      },
      Statistic: {
        contentFontSize: 28,
      },
      Button: {
        controlHeight: 36,
        fontWeight: 500,
      },
    },
  };
}

export const appTheme = createAppTheme('light');
