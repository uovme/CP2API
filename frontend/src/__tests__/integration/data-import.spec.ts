import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'

const mocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  importData: vi.fn(),
  importCLIProxyAuth: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: mocks.showError,
    showSuccess: mocks.showSuccess
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      importData: mocks.importData,
      importCLIProxyAuth: mocks.importCLIProxyAuth
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('ImportDataModal', () => {
  beforeEach(() => {
    mocks.showError.mockReset()
    mocks.showSuccess.mockReset()
    mocks.importData.mockReset()
    mocks.importCLIProxyAuth.mockReset()
  })

  it('未选择文件时提示错误', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    await wrapper.find('form').trigger('submit')
    expect(mocks.showError).toHaveBeenCalledWith('admin.accounts.dataImportSelectFile')
  })

  it('无效 JSON 时提示解析失败', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const file = new File(['invalid json'], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve('invalid json')
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await Promise.resolve()

    expect(mocks.showError).toHaveBeenCalledWith('admin.accounts.dataImportParseFailed')
  })

  it('选择 CLIProxyAPI auth 格式时按原始文本导入', async () => {
    mocks.importCLIProxyAuth.mockResolvedValue({
      total: 1,
      created: 1,
      updated: 0,
      skipped: 0,
      failed: 0
    })

    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    expect(wrapper.find('input[value="cliproxy_auth"]').exists()).toBe(true)
    await wrapper.find('input[value="cliproxy_auth"]').setValue(true)

    const input = wrapper.find('input[type="file"]')
    const content = '{"type":"claude","refresh_token":"rt"}'
    const file = new File([content], 'auth.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve(content)
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await Promise.resolve()

    expect(mocks.importCLIProxyAuth).toHaveBeenCalledWith({
      content,
      skip_default_group_bind: true,
      update_existing: true
    })
    expect(mocks.importData).not.toHaveBeenCalled()
    expect(mocks.showSuccess).toHaveBeenCalledWith('admin.accounts.cliProxyAuthImportSuccess')
  })
})
