# elk-alarm-notifier
Forward Kibana configured fired alarms to external system. As a reference we are focusing to logs-threashold alert type described at https://www.elastic.co/guide/en/observability/7.17/logs-threshold-alert.html .

At the moment we suggest, and support, the following document mapping to be indexed:

```json
{
  "ruleId": "{{rule.id}}",
  "ruleName": "{{rule.name}}",
  "alertId": "{{alert.id}}",
  "contextMessage": "{{context.message}}",
  "event": "fired",
  "contextMatchingDocuments": "{{context.matchingDocuments}}",
  "@timestamp": "{{context.timestamp}}",
  "kibanaBaseUrl": "{{kibanaBaseUrl}}",
  "tags": "{{rule.tags}}"
}
```

You can set `event` attribute in a statically way to distinguish between `fired` and `recovered` events.

## Configuration

- `ELASTIC_HOST`: ElasticSearch host. Default to `https://localhost:9200`. Only single host supported at the moment.
- `ELASTIC_USERNAME`: Username for ElasticSearch
- `ELASTIC_PASSWORD`: Password for ElasticSearch
- `ELASTIC_INDEX`: Index where Kibana alerts will be stored
- `ELASTIC_TIMESTAMP_FIELD`: Range query using `gte` will be based on that field. Default to `@timestamp`
- `ELASTIC_TAGS_FIELD`: Field to holds tags. Default to `tags`
- `ELASTIC_EVENT_TYPE_FIELD`: Extra field used to manage `fired` vs `recovered` alarm status. Default to `event`.
- `NOTIFY_CHANNEL`: Channel to be used to deliver notification. Only `msteams` for Microsoft Teams supported at the moment.
- `NOTIFY_MSTEAMS_WEBHOOK`: Webhook to reach MS Teams Channel.
- `ALERT_INTERVAL`: How often search for new events in seconds. Default to 300 seconds (5 mins)
- `DRYRUN`: To send or not notifications. If is set to `true` the message will be only written in logs.

## Deployment

This project is inteded to be executed on Kubernetes via a standard Deployment. To deploy the easy way is to apply a manifest like the following:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: elk-alarm-notifier
spec:
  replicas: 1
  revisionHistoryLimit: 3
  strategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app: elk-alarm-notifier
  template:
    metadata:
      labels:
        app: elk-alarm-notifier
    spec:
      containers:
      - name: elk-alarm-notifier
        imagePullPolicy: Always
        image: ghcr.io/acarmisc/elk-alarm-notifier:release
        env:
          - name: ELASTIC_HOST
            value: "https://somehost:9200"
          - name: ELASTIC_USERNAME
            value: "elastic"
          - name: ELASTIC_PASSWORD
            value: "s3cr3t"
          - name: ELASTIC_INDEX
            value: "alarms"          
          - name: NOTIFY_MSTEAMS_WEBHOOK
            value: "https://idoqsrl.webhook.office.com/webhookb2/someHash"          
          - name: DRYRUN
            value: "false"
        resources:
          requests:
            cpu: 50m
            memory: 128Mi
          limits:
            cpu: 100m
            memory: 256Mi        


```