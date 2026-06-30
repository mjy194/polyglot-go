import type { CSSProperties } from 'react';
import { Button, Tooltip } from 'antd';
import { MoonOutlined, SunOutlined } from '@ant-design/icons';
import { useThemeMode } from '../context/ThemeContext';

interface Props {
  style?: CSSProperties;
}

function ThemeToggleButton({ style }: Props) {
  const { mode, toggleMode } = useThemeMode();
  const currentLabel = mode === 'dark' ? '深色' : '浅色';
  const nextLabel = mode === 'dark' ? '浅色' : '深色';
  const label = `当前${currentLabel},点击切换到${nextLabel}`;

  return (
    <Tooltip title={label}>
      <Button
        type="text"
        aria-label={label}
        aria-pressed={mode === 'dark'}
        icon={mode === 'dark' ? <MoonOutlined /> : <SunOutlined />}
        onClick={toggleMode}
        style={{ fontSize: 16, ...style }}
      />
    </Tooltip>
  );
}

export default ThemeToggleButton;
