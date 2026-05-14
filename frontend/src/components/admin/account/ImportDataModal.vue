<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.dataImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="import-data-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.dataImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-600 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ importFormat === 'cliproxy_auth' ? t('admin.accounts.cliProxyAuthImportWarning') : t('admin.accounts.dataImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.dataImportFormat') }}</label>
        <div class="grid gap-2 sm:grid-cols-2">
          <label
            class="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3 text-sm transition-colors hover:border-primary-300 dark:border-dark-700 dark:hover:border-primary-700"
            :class="importFormat === 'sub2api' ? 'border-primary-500 bg-primary-50 dark:border-primary-500 dark:bg-primary-900/20' : 'bg-white dark:bg-dark-800'"
          >
            <input
              v-model="importFormat"
              type="radio"
              class="mt-1 h-4 w-4 border-gray-300 text-primary-600 focus:ring-primary-500"
              value="sub2api"
            />
            <span class="min-w-0">
              <span class="block font-medium text-gray-900 dark:text-white">{{ t('admin.accounts.sub2apiImportFormat') }}</span>
              <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.accounts.sub2apiImportFormatHint') }}</span>
            </span>
          </label>
          <label
            class="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3 text-sm transition-colors hover:border-primary-300 dark:border-dark-700 dark:hover:border-primary-700"
            :class="importFormat === 'cliproxy_auth' ? 'border-primary-500 bg-primary-50 dark:border-primary-500 dark:bg-primary-900/20' : 'bg-white dark:bg-dark-800'"
          >
            <input
              v-model="importFormat"
              type="radio"
              class="mt-1 h-4 w-4 border-gray-300 text-primary-600 focus:ring-primary-500"
              value="cliproxy_auth"
            />
            <span class="min-w-0">
              <span class="block font-medium text-gray-900 dark:text-white">{{ t('admin.accounts.cliProxyAuthImportFormat') }}</span>
              <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.accounts.cliProxyAuthImportFormatHint') }}</span>
            </span>
          </label>
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.dataImportFile') }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200">
              {{ fileName || t('admin.accounts.dataImportSelectFile') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">{{ fileTypeHint }}</div>
          </div>
          <button type="button" class="btn btn-secondary shrink-0" @click="openFilePicker">
            {{ t('common.chooseFile') }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          :accept="fileAccept"
          @change="handleFileChange"
        />
      </div>

      <div
        v-if="result"
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.dataImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ resultSummary }}
        </div>

        <div v-if="errorItems.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('admin.accounts.dataImportErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
          >
            <div v-for="(item, idx) in errorItems" :key="idx" class="whitespace-pre-wrap">
              {{ itemLabel(item) }} - {{ item.message }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="import-data-form"
          :disabled="importing"
        >
          {{ importing ? t('admin.accounts.dataImporting') : t('admin.accounts.dataImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type {
  AdminDataImportError,
  AdminDataImportResult,
  CLIProxyAuthImportMessage,
  CLIProxyAuthImportResult
} from '@/types'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'imported'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

type ImportFormat = 'sub2api' | 'cliproxy_auth'
type ImportResult =
  | { format: 'sub2api'; data: AdminDataImportResult }
  | { format: 'cliproxy_auth'; data: CLIProxyAuthImportResult }
type ImportErrorItem = AdminDataImportError | CLIProxyAuthImportMessage

const importing = ref(false)
const file = ref<File | null>(null)
const importFormat = ref<ImportFormat>('sub2api')
const result = ref<ImportResult | null>(null)

const fileInput = ref<HTMLInputElement | null>(null)
const fileName = computed(() => file.value?.name || '')
const fileAccept = computed(() => importFormat.value === 'cliproxy_auth' ? 'application/json,.json,.txt' : 'application/json,.json')
const fileTypeHint = computed(() => importFormat.value === 'cliproxy_auth' ? t('admin.accounts.cliProxyAuthImportFileHint') : 'JSON (.json)')

const resultSummary = computed(() => {
  if (!result.value) return ''
  if (result.value.format === 'cliproxy_auth') {
    return t('admin.accounts.cliProxyAuthImportResultSummary', result.value.data)
  }
  return t('admin.accounts.dataImportResultSummary', result.value.data)
})

const errorItems = computed<ImportErrorItem[]>(() => result.value?.data.errors || [])

watch(
  () => props.show,
  (open) => {
    if (open) {
      file.value = null
      result.value = null
      importFormat.value = 'sub2api'
      if (fileInput.value) {
        fileInput.value.value = ''
      }
    }
  }
)

const openFilePicker = () => {
  fileInput.value?.click()
}

const handleFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement
  file.value = target.files?.[0] || null
  result.value = null
}

const handleClose = () => {
  if (importing.value) return
  emit('close')
}

const readFileAsText = async (sourceFile: File): Promise<string> => {
  if (typeof sourceFile.text === 'function') {
    return sourceFile.text()
  }

  if (typeof sourceFile.arrayBuffer === 'function') {
    const buffer = await sourceFile.arrayBuffer()
    return new TextDecoder().decode(buffer)
  }

  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error || new Error('Failed to read file'))
    reader.readAsText(sourceFile)
  })
}

const itemLabel = (item: ImportErrorItem): string => {
  const maybeDataItem = item as AdminDataImportError
  if (maybeDataItem.kind || maybeDataItem.proxy_key) {
    return `${maybeDataItem.kind || 'account'} ${maybeDataItem.name || maybeDataItem.proxy_key || '-'}`
  }

  const maybeAuthItem = item as CLIProxyAuthImportMessage
  return `#${maybeAuthItem.index} ${maybeAuthItem.name || '-'}`
}

const handleSub2APIImport = async (text: string) => {
  const dataPayload = JSON.parse(text)
  const res = await adminAPI.accounts.importData({
    data: dataPayload,
    skip_default_group_bind: true
  })

  result.value = { format: 'sub2api', data: res }

  const msgParams: Record<string, unknown> = {
    account_created: res.account_created,
    account_failed: res.account_failed,
    proxy_created: res.proxy_created,
    proxy_reused: res.proxy_reused,
    proxy_failed: res.proxy_failed,
  }
  if (res.account_failed > 0 || res.proxy_failed > 0) {
    appStore.showError(t('admin.accounts.dataImportCompletedWithErrors', msgParams))
  } else {
    appStore.showSuccess(t('admin.accounts.dataImportSuccess', msgParams))
    emit('imported')
  }
}

const handleCLIProxyAuthImport = async (text: string) => {
  const res = await adminAPI.accounts.importCLIProxyAuth({
    content: text,
    skip_default_group_bind: true,
    update_existing: true
  })

  result.value = { format: 'cliproxy_auth', data: res }

  const msgParams: Record<string, unknown> = {
    total: res.total,
    created: res.created,
    updated: res.updated,
    skipped: res.skipped,
    failed: res.failed
  }
  if (res.failed > 0) {
    appStore.showError(t('admin.accounts.cliProxyAuthImportPartial', msgParams))
  } else {
    appStore.showSuccess(t('admin.accounts.cliProxyAuthImportSuccess', msgParams))
    emit('imported')
  }
}

const handleImport = async () => {
  if (!file.value) {
    appStore.showError(t('admin.accounts.dataImportSelectFile'))
    return
  }

  importing.value = true
  try {
    const text = await readFileAsText(file.value)
    if (importFormat.value === 'cliproxy_auth') {
      await handleCLIProxyAuthImport(text)
    } else {
      await handleSub2APIImport(text)
    }
  } catch (error: any) {
    if (error instanceof SyntaxError) {
      appStore.showError(t('admin.accounts.dataImportParseFailed'))
    } else {
      appStore.showError(error?.message || t('admin.accounts.dataImportFailed'))
    }
  } finally {
    importing.value = false
  }
}
</script>
