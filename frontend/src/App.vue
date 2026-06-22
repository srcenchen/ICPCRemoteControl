<script setup lang="ts">
import { RouterView } from 'vue-router'
import { useToast } from '@/composables/useToast'
import ToastContainer from '@/components/ToastContainer.vue'

const { toasts, remove } = useToast()
</script>

<template>
  <RouterView />
  <ToastContainer :toasts="toasts" @remove="remove" />
</template>
