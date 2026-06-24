import { Form, Input } from 'antd';
import { listModelMappings, upsertModelMapping } from '../api/client';
import { useCrud } from '../hooks/useCrud';
import PageContainer from '../components/PageContainer';
import CrudTable from '../components/CrudTable';
import EntityFormModal from '../components/EntityFormModal';
import type { ModelMapping } from '../api/types';

function ModelMappings() {
  const crud = useCrud<ModelMapping>({
    list: () => listModelMappings(),
    save: upsertModelMapping,
    deps: [],
    defaults: {} as Partial<ModelMapping>,
  });

  return (
    <PageContainer description="把请求模型名映射到上游实际模型">
      <CrudTable<ModelMapping>
        data={crud.data}
        loading={crud.loading}
        onReload={crud.reload}
        onCreate={crud.openCreate}
        createText="新增映射"
        columns={[
          { title: 'Provider ID', dataIndex: 'provider_id' },
          {
            title: '源模型',
            dataIndex: 'from_model',
            sorter: (a, b) => a.from_model.localeCompare(b.from_model),
          },
          { title: '目标模型', dataIndex: 'to_model' },
          {
            title: '操作',
            fixed: 'right',
            render: (_, r) => <a onClick={() => crud.openEdit(r)}>编辑</a>,
          },
        ]}
      />

      <EntityFormModal
        title={crud.isEditing ? '编辑映射' : '新增映射'}
        open={crud.open}
        form={crud.form}
        saving={crud.saving}
        onOk={crud.submit}
        onCancel={crud.closeModal}
      >
        <Form.Item name="id" hidden>
          <Input />
        </Form.Item>
        <Form.Item label="Provider ID" name="provider_id" rules={[{ required: true }]}>
          <Input />
        </Form.Item>
        <Form.Item label="源模型" name="from_model" rules={[{ required: true }]}>
          <Input placeholder="gpt-4" />
        </Form.Item>
        <Form.Item label="目标模型" name="to_model" rules={[{ required: true }]}>
          <Input placeholder="claude-sonnet-4-6" />
        </Form.Item>
      </EntityFormModal>
    </PageContainer>
  );
}

export default ModelMappings;
