<script setup lang="ts">
interface ToastEntry {
  id: number;
  message: string;
  tone: "success" | "error" | "info";
}

const { toasts } = defineProps<{
  toasts: ToastEntry[];
}>();

function toneClass(tone: ToastEntry["tone"]) {
  if (tone === "success") {
    return "ui-toast ui-toast-success";
  }

  if (tone === "error") {
    return "ui-toast ui-toast-danger";
  }

  return "ui-toast ui-toast-info";
}
</script>

<template>
  <div class="pointer-events-none fixed inset-x-4 top-4 z-[80] flex flex-col gap-2 lg:inset-x-auto lg:right-6 lg:top-auto lg:bottom-6 lg:w-80">
    <TransitionGroup name="toast">
      <div v-for="toast in toasts" :key="toast.id" :class="toneClass(toast.tone)" class="rounded-2xl px-4 py-3 text-sm font-medium backdrop-blur">
        {{ toast.message }}
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.25s ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-12px);
}

@media (min-width: 1024px) {
  .toast-enter-from,
  .toast-leave-to {
    transform: translateX(20px);
  }
}
</style>
