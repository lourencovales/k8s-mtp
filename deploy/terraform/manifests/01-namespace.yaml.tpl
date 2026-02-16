apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
  labels:
    app.kubernetes.io/name: k8s-mtp
    app.kubernetes.io/component: database
