import { ReactNode } from 'react';
import { Modal, Form } from 'antd';
import type { FormInstance } from 'antd';

interface Props {
  title: string;
  open: boolean;
  form: FormInstance;
  saving?: boolean;
  onOk: () => void;
  onCancel: () => void;
  children: ReactNode;
}

// 统一增改弹窗:Modal + 垂直 Form + 保存 loading
function EntityFormModal({ title, open, form, saving, onOk, onCancel, children }: Props) {
  return (
    <Modal
      title={title}
      open={open}
      onOk={onOk}
      confirmLoading={saving}
      onCancel={onCancel}
      destroyOnClose
      maskClosable={false}
    >
      <Form form={form} layout="vertical" style={{ marginTop: 12 }}>
        {children}
      </Form>
    </Modal>
  );
}

export default EntityFormModal;
