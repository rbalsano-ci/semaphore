<template>
  <div class="padded">
    <v-dialog
      v-model="hostsDialog"
      :max-width="800"
      persistent
      :transition="false"
    >
      <div v-if="!isLoaded">
        <v-progress-linear
          indeterminate
          color="primary darken-2"
        ></v-progress-linear>
      </div>
      <div v-else>
        Click on a MAC address listed below to select the given device.
        <v-data-table
          :headers="headers"
          :items="items"
          class="mt-4"
        >

          <template v-slot:item.Mac="{ item }">
            <v-btn @click="selectHost(item.Mac, item.Ip)">{{ item.Mac }}</v-btn>
          </template>

          <template v-slot:item.Ip="{ item }">
            {{ item.Ip }}
          </template>

          <template v-slot:item.Vendor="{ item }">
            {{ item.Vendor }}
          </template>

        </v-data-table>
        <v-btn @click="hostsDialog = false">Cancel</v-btn>
      </div>
    </v-dialog>

    <v-form ref="form">
      <h1 class="step">1. Device Identification</h1>
      Search LAN for devices:
      <v-btn icon @click="hostsDialog = true">
        <v-icon>mdi-lan</v-icon>
      </v-btn>
      or manually enter the MAC address and name (the IP address will then be looked up).

      <v-text-field
        v-model="macAddress"
        label="MAC Address"
        placeholder="01:23:45:67:89:AB"
      />

      <v-text-field
        v-model="hostname"
        label="Name"
        placeholder="Name"
      />

      <div v-if="!!ipAddress">
        IP Address: {{ ipAddress }}
      </div>

      <h1 class="step">2. Public Key Infrastructure (PKI) Files</h1>
      <div v-if="existingPki">
        Found {{ existingPkiValid ? 'valid' : 'invalid' }} PKI files for
        MAC address: {{ macAddress }}
        <ul>
          <li v-if="existingKeyFileName != ''">
            {{ existingKeyFileName }}
            <v-btn icon @click="deleteFile(existingKeyFileName)">
              <v-icon>mdi-delete</v-icon>
            </v-btn>
          </li>
          <li v-if="existingCrtFileName != ''">
            {{ existingCrtFileName }} (Certificate Subject: {{ existingCrtSubject }})
            <v-btn icon @click="deleteFile(existingCrtFileName)">
              <v-icon>mdi-delete</v-icon>
            </v-btn>
          </li>
        </ul>
        {{ this.existingPkiInvalidReason }}
        Upload new .key or .crt files below.
      </div>
      <div v-else>
        <div v-if="!!macAddress">
          No existing .key or .crt files found for MAC address: {{ macAddress }}; upload below.
        </div>
      </div>
      <v-file-input accept=".key," prepend-icon="mdi-key" v-model="keyFile"/>
      <v-file-input accept=".crt," prepend-icon="mdi-certificate" v-model="crtFile"/>
      <v-btn @click="uploadFiles()">Upload</v-btn>
      <div v-if="readyToProvision() || taskSuccessful()">
        <h1 class="step">3. Finalize</h1>
        <v-btn v-if="readyToProvision() && !taskSuccessful()" @click="provision()">Provision</v-btn>
        <v-btn v-if="taskSuccessful()" @click="cleanUp()">Clean up</v-btn>
      </div>
    </v-form>
  </div>
</template>

<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';
import ItemListPageBase from '@/components/ItemListPageBase';
import EventBus from '@/event-bus';

export default {
  props: {
    projectId: Number,
  },

  mixins: [ItemListPageBase],

  data() {
    return {
      templateName: 'Provision - Bootstrap',
      templateId: null,
      inventoryFileContents: '',
      hostsDialog: false,
      hostname: null,
      macAddress: null,
      ipAddress: null,
      taskId: null,
      taskStatus: null,
      keyFile: null,
      crtFile: null,
      existingPki: false,
      existingPkiValid: false,
      existingKeyFileName: '',
      existingCrtFileName: '',
      existingCrtSubject: '',
      existingPkiInvalidReason: '',
    };
  },
  computed: {
    isLoaded() {
      return this.items;
    },
  },
  watch: {
    async templateId() {
      await this.getInventoryContents();
    },

    async inventoryFileContents() {
      const match = this.inventoryFileContents.match(
        /(\S+)\s*ansible_host=((?:\d{1,3}\.){3,3}\d{1,3})/,
      );
      if ((match != null) && (match.length >= 3)) {
        this.hostname = match[1];
        this.ipAddress = match[2];
        await this.setMacAddress();
      }
    },
    async ipAddress() {
      this.updateInventoryFile();
    },
    async hostname() {
      this.updateInventoryFile();
    },
    async macAddress() {
      await this.ipLookup();
      await this.summarizePkiStatus();
    },
  },
  methods: {
    async getTemplateId() {
      const template = (await axios({
        method: 'get',
        url: `/api/project/${this.projectId}/templates/`,
        responseType: 'json',
      })).data.filter((t) => t.name === this.templateName)[0];
      if (template == null) {
        return;
      }
      this.templateId = template.id;
    },
    async getInventoryContents() {
      if (this.templateId == null) {
        this.inventoryFileContents = '';
        return;
      }

      this.inventoryFileContents = (await axios({
        keys: 'method',
        url: `/api/project/${this.projectId}/templates/${this.templateId}/inventory_contents/`,
        responseType: 'text',
      })).data;
    },

    async updateInventoryFile() {
      await axios({
        method: 'put',
        url: `/api/project/${this.projectId}/templates/${this.templateId}/inventory_contents/`,
        responseType: 'json',
        data: {
          mac: this.macAddress,
          ip: this.ipAddress,
          name: this.hostname,
        },
      });
    },

    async ipLookup() {
      try {
        if (this.macAddress) {
          this.ipAddress = null;
          const response = (await axios({
            method: 'get',
            url: `/api/network/ip/${this.macAddress}`,
            responseType: 'json',
          }));
          this.ipAddress = response.data.ip;
        }
      } catch (err) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: getErrorMessage(err),
        });
      }
    },

    getHeaders() {
      return [
        {
          text: 'MAC Address',
          value: 'Mac',
          sortable: true,
        },
        {
          text: 'IP Address',
          value: 'Ip',
          sortable: true,
        },
        {
          text: 'Network Card Vendor',
          value: 'Vendor',
          sortable: true,
        },
      ];
    },

    async updateTaskStatus() {
      if (this.taskId) {
        const response = (await axios({
          method: 'get',
          url: `/api/project/${this.projectId}/tasks/${this.taskId}`,
          responseType: 'json',
        }));
        this.taskStatus = response.data.status;
      }
    },

    getItemsUrl() {
      return '/api/network/local_hosts';
    },

    selectHost(mac, ip) {
      this.macAddress = mac;
      this.ipAddress = ip;
      this.hostsDialog = false;
    },

    showTaskLog() {
      EventBus.$emit('i-show-task', {
        taskId: this.taskId,
      });
    },
    async setMacAddress() {
      try {
        if (this.ipAddress) {
          this.macAddress = null;
          const response = (await axios({
            method: 'get',
            url: `/api/network/mac/${this.ipAddress}`,
            responseType: 'json',
          }));
          this.macAddress = response.data.mac;
        }
      } catch (err) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: getErrorMessage(err),
        });
      }
    },
    clearPkiVars() {
      this.existingPki = false;
      this.existingPkiValid = false;
      this.existingKeyFileName = '';
      this.existingCrtFileName = '';
      this.existingCrtSubject = '';
      this.existingPkiInvalidReason = '';
    },
    async summarizePkiStatus() {
      try {
        if (this.macAddress) {
          const response = (await axios({
            method: 'get',
            url: `/api/project/${this.projectId}/templates/${this.templateId}/pki/${this.macAddress}`,
            responseType: 'json',
          }));
          if (response.status === 200) {
            const d = response.data;

            this.existingPki = true;
            this.existingPkiValid = d.Valid;
            this.existingKeyFileName = d.KeyName;
            this.existingCrtFileName = d.CertName;
            this.existingCrtSubject = d.CertSubject;
            this.existingPkiInvalidReason = d.Valid ? '' : (`Reason marked invalid: ${d.InvalidReason}`);
          }
        } else {
          this.clearPkiVars();
        }
      } catch (err) {
        this.clearPkiVars();
      }
    },
    async uploadFiles() {
      const data = new FormData();

      if (!(this.keyFile.toString() === '')) {
        data.append('key-file', this.keyFile);
      }
      if (!(this.crtFile.toString() === '')) {
        data.append('crt-file', this.crtFile);
      }

      await axios({
        method: 'post',
        url: `/api/project/${this.projectId}/templates/${this.templateId}/pki/${this.macAddress}`,
        responseType: 'json',
        data,
      });
      await this.summarizePkiStatus();
    },
    async deleteFile(fileName) {
      await axios({
        method: 'delete',
        url: `/api/project/${this.projectId}/templates/${this.templateId}/pki/${this.macAddress}/${fileName}`,
        responseType: 'json',
      });
      await this.summarizePkiStatus();
    },
    async cleanUp() {
      try {
        await axios({
          method: 'delete',
          url: `/api/project/${this.projectId}/templates/${this.templateId}/inventory_contents`,
          responseType: 'json',
        });
        await axios({
          method: 'delete',
          url: `/api/project/${this.projectId}/templates/${this.templateId}/pki/${this.macAddress}`,
          responseType: 'json',
        });
        this.hostname = null;
        this.macAddress = null;
        this.ipAddress = null;
        this.keyFile = null;
        this.crtFile = null;
        this.taskId = null;
        this.taskStatus = null;
      } catch (err) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: getErrorMessage(err),
        });
      }
    },

    async provision() {
      try {
        (await axios({
          method: 'get',
          url: `/api/network/prepare_ssh/${this.ipAddress}`,
        }));
        const response = (await axios({
          method: 'post',
          url: `/api/project/${this.projectId}/tasks`,
          responseType: 'json',
          data: {
            environment: '{}',
            project_id: this.projectId,
            template_id: this.templateId,
          },
        }));
        this.taskId = response.data.id;
      } catch (err) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: getErrorMessage(err),
        });
      }

      this.showTaskLog();
    },

    readyToProvision() {
      return this.existingPkiValid && !!this.hostname && !!this.macAddress;
    },

    taskSuccessful() {
      return this.taskStatus === 'success';
    },
  },
  async created() {
    await this.getTemplateId();
  },
  mounted() {
    EventBus.$on('i-task-log-closed', async (e) => {
      if (e.closed) {
        await this.updateTaskStatus();
      }
    });
  },
};
</script>

<style lang="scss">

div.padded {
  padding: 20px;
}

h1.step {
  color: rgba(0, 0, 0, 0.6);
  text-transform: uppercase;
  height: 48px;
  font-size: 1rem !important;
  font-weight: bold;
  padding: 40px;
}
</style>
