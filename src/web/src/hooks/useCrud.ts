import { useState } from 'react';
import { Form, App } from 'antd';
import { useFetch } from './useFetch';

interface CrudOptions<T> {
  list: () => Promise<T[]>;
  save: (values: Partial<T>) => Promise<unknown>;
  deps?: unknown[];
  defaults?: Partial<T>;
}

// 封装"列表 + 新建/编辑弹窗 + 保存"的通用状态机,压缩各页样板。
export function useCrud<T extends object>(opts: CrudOptions<T>) {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const { data, loading, reload } = useFetch<T[]>(opts.list, opts.deps ?? []);
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<T | undefined>();

  const openCreate = () => {
    setEditing(undefined);
    form.resetFields();
    if (opts.defaults) form.setFieldsValue(opts.defaults);
    setOpen(true);
  };

  const openEdit = (row: T) => {
    setEditing(row);
    form.resetFields();
    form.setFieldsValue(row);
    setOpen(true);
  };

  const submit = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const saved = await opts.save(values);
      message.success('已保存');
      setOpen(false);
      reload();
      return saved;
    } catch (e: any) {
      message.error(e?.response?.data?.error || e?.message || '保存失败');
      return undefined;
    } finally {
      setSaving(false);
    }
  };

  return {
    data,
    loading,
    reload,
    form,
    open,
    saving,
    editing,
    isEditing: !!editing,
    openCreate,
    openEdit,
    closeModal: () => setOpen(false),
    submit,
  };
}
