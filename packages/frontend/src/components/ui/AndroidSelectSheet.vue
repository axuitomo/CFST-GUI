<script setup vapor lang="ts">
import type { AndroidSelectOption } from "../../composables/useAndroidSelectPicker";

defineProps<{
  open: boolean;
  options: AndroidSelectOption[];
  title: string;
  value: string;
}>();

defineEmits<{
  (event: "choose", option: AndroidSelectOption): void;
  (event: "close"): void;
}>();
</script>

<template>
  <div v-if="open" class="android-select-overlay" role="presentation" @click.self="$emit('close')">
    <section class="android-select-sheet" role="dialog" aria-modal="true" :aria-label="title">
      <div class="android-select-header">
        <h2 class="truncate text-base font-semibold">{{ title }}</h2>
        <button type="button" class="android-select-close" aria-label="关闭" @click="$emit('close')">×</button>
      </div>
      <div class="android-select-options" role="listbox">
        <button
          v-for="option in options"
          :key="`${option.value}:${option.label}`"
          type="button"
          class="android-select-option"
          :class="option.selected || option.value === value ? 'android-select-option-active' : ''"
          :disabled="option.disabled"
          role="option"
          :aria-selected="option.selected || option.value === value"
          @click="$emit('choose', option)"
        >
          <span class="truncate">{{ option.label }}</span>
          <span v-if="option.selected || option.value === value" class="android-select-check">✓</span>
        </button>
      </div>
    </section>
  </div>
</template>
