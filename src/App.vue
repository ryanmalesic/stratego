<script setup lang="ts">
import { Authenticator } from "@aws-amplify/ui-vue";
import { RouterView } from "vue-router";

import "@aws-amplify/ui-vue/styles.css";
import Auth from "@aws-amplify/auth";

Auth.currentCredentials().then((info) => {
  console.log(info.identityId);
});
</script>

<template>
  <authenticator>
    <template v-slot="{}">
      <RouterView />
    </template>
  </authenticator>
</template>
