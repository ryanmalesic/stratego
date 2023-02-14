import { createApp, markRaw } from "vue";
import { createPinia } from "pinia";
import { Amplify } from "@aws-amplify/core";
import { AWSIoTProvider } from "@aws-amplify/pubsub";

import App from "./App.vue";
import awsconfig from "./aws-exports";
import router from "./router/index";

import "./assets/main.css";


Amplify.configure(awsconfig);

Amplify.configure({
  aws_cloud_logic_custom: [
    {
      ...awsconfig.aws_cloud_logic_custom[0],
      endpoint: import.meta.env.PROD
        ? awsconfig.aws_cloud_logic_custom[0].endpoint
        : "http://localhost:8080",
    },
  ],
});

Amplify.addPluggable(
  new AWSIoTProvider({
    aws_pubsub_region: 'us-west-2',
    aws_pubsub_endpoint:
      'wss://a29k7932wdwned-ats.iot.us-west-2.amazonaws.com/mqtt'
  })
);

const app = createApp(App);
const pinia = createPinia();
pinia.use(({ store }) => {
  store.$router = markRaw(router)
});
app.use(pinia);
app.use(router);


app.mount("#app");
