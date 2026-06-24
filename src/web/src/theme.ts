import { theme } from 'antd';
import type { ThemeConfig } from 'antd';

// 统一设计 token —— 现代 SaaS 控制台风格(indigo 主色 + 中性灰 + 弱阴影)。
// 一处定义,全局生效,页面/组件不再写零散 style。
export const appTheme: ThemeConfig = {
  algorithm: theme.defaultAlgorithm,
  token: {
    colorPrimary: '#4f46e5',
    colorInfo: '#4f46e5',
    colorSuccess: '#16a34a',
    colorWarning: '#d97706',
    colorError: '#dc2626',
    colorBgLayout: '#f7f8fa',
    colorBorderSecondary: '#eef0f3',
    borderRadius: 8,
    fontSize: 14,
    wireframe: false,
    fontFamily:
      "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
  },
  components: {
    Layout: {
      headerBg: '#ffffff',
      headerHeight: 56,
      headerPadding: '0 20px',
      bodyBg: '#f7f8fa',
      siderBg: '#0f172a',
    },
    Menu: {
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
      boxShadowTertiary: '0 1px 2px rgba(16,24,40,0.06), 0 1px 3px rgba(16,24,40,0.04)',
    },
    Table: {
      headerBg: '#fafafa',
      headerColor: '#475467',
      rowHoverBg: '#f9fafb',
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
