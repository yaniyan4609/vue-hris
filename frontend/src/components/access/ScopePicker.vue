<script setup>
import { Plus, Trash2 } from 'lucide-vue-next'
import BaseButton from '../ui/BaseButton.vue'
import BaseMultiSelect from '../ui/BaseMultiSelect.vue'
import BaseSearchableSelect from '../ui/BaseSearchableSelect.vue'

const props = defineProps({ modelValue: { type: Array, default: () => [] }, options: { type: Object, default: () => ({}) } })
const emit = defineEmits(['update:modelValue'])
const dimensions = [{ key: 'regions', label: 'Wilayah' }, { key: 'offices', label: 'Kantor' }, { key: 'divisions', label: 'Divisi' }, { key: 'departments', label: 'Departemen' }, { key: 'positions', label: 'Jabatan' }]
const kindOptions = [{ value: 'allTenant', label: 'Seluruh Tenant' }, { value: 'self', label: 'Diri Sendiri' }, { value: 'directReports', label: 'Bawahan Langsung' }, { value: 'filter', label: 'Struktur Organisasi' }]
function kind(rule) { return rule.allTenant ? 'allTenant' : rule.self ? 'self' : rule.directReports ? 'directReports' : 'filter' }
function update(index, value) { const next = props.modelValue.map((rule) => ({ ...rule })); next[index] = value; emit('update:modelValue', next) }
function changeKind(index, value) { update(index, value === 'filter' ? { regions: [], offices: [], divisions: [], departments: [], positions: [] } : { [value]: true }) }
function changeDimension(index, key, values) { update(index, { ...props.modelValue[index], [key]: values }) }
function add() { emit('update:modelValue', [...props.modelValue, { allTenant: true }]) }
function remove(index) { emit('update:modelValue', props.modelValue.filter((_, i) => i !== index)) }
</script>

<template>
  <div class="space-y-3"><p class="text-xs text-neutral-500 dark:text-neutral-400">Beberapa nilai dalam satu dimensi digabung OR, antardimensi dalam satu aturan AND, dan antaraturan OR.</p>
    <div v-for="(rule, index) in modelValue" :key="index" class="rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-neutral-50 dark:bg-[var(--theme-surface-raised)] p-3 space-y-3"><div class="flex items-center gap-2"><span class="text-xs font-semibold flex-1">Aturan {{ index + 1 }}</span><BaseButton variant="outline" size="sm" title="Hapus aturan" @click="remove(index)"><Trash2 class="w-3.5 h-3.5" /></BaseButton></div><BaseSearchableSelect :model-value="kind(rule)" :options="kindOptions" :searchable="false" size="md" @update:model-value="changeKind(index, $event)" /><div v-if="kind(rule) === 'filter'" class="grid sm:grid-cols-2 gap-2"><BaseMultiSelect v-for="dimension in dimensions" :key="dimension.key" :model-value="rule[dimension.key] || []" :options="options[dimension.key] || []" :label="dimension.label" :placeholder="`Pilih ${dimension.label.toLowerCase()}`" @update:model-value="changeDimension(index, dimension.key, $event)" /></div></div>
    <BaseButton variant="outline" size="sm" @click="add"><Plus class="w-4 h-4" />Tambah aturan OR</BaseButton>
  </div>
</template>
