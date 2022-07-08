<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>
    <v-toolbar flat color="white">
      <v-toolbar-title>
        Provisioning
      </v-toolbar-title>
    </v-toolbar>
    <Provision :project-id="projectId"/>
  </div>
</template>

<script>
import Provision from '@/components/Provision.vue';

export default {
  props: {
    projectId: Number,
  },
  components: { Provision },
};
</script>
