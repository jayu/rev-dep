<template>
  <main>{{ message }}</main>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { secret } from "./private";
import { greet } from "./utils";
import missing from "non-existent-vue-pkg";

const message = ref(greet(secret + String(missing)));
</script>

<style scoped>
main {
  font-weight: 600;
}
</style>
