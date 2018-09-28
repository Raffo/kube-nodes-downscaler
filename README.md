# Kube Nodes Downscaler

Running clusters in highly dynamic contexts means that we often run stuff we don't really need.
Wasting money is stupid and we really want to make sure that we only run stuff when we need to.
This little piece of code just scale down ASGs based on day and time to allow us to have clusters that are available only during working hours.

## Mode of executions

The options should be self explanatory, if not please submit a bug report. There are however different options to deploy:

- run it on a master node: in this way we will need to pass the ASG name as parameter (we never want to scale down to 0 the master!)
- run it on a worker node: in this case we can use the autodiscovery

An example deployment file is the following:

```
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  namespace: kube-system
  name: downscaler
  labels:
    k8s-app: downscaler
spec:
  template:
    metadata:
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ""
      labels:
        k8s-app: downscaler
    spec:
      # run on each master node
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - effect: NoSchedule
        key: node-role.kubernetes.io/master
      - key: CriticalAddonsOnly
        operator: Exists
      hostNetwork: true
      containers:
      - name: downscaler
        image: x0rg/kube-nodes-downscaler:v0.0.5
        imagePullPolicy: Always
        args:
          - --start=7
          - --end=17
          - --asg-name=ASGN_NAME
        resources:
          requests:
            memory: 20Mi
            cpu: 10m
          limits:
            memory: 20Mi
            cpu: 100m
```

Images are provided on Docker Hub. Images with the `latest` tag are not publish to be able to easily figure out what is running in production. You can find what is the latest imaget tag by looking at [this page](https://hub.docker.com/r/x0rg/kube-nodes-downscaler/tags/).

## Helm chart

A [Helm](https://helm.sh/) chart is available. You can install it using:

```bash
helm repo add raffo-kube-nodes-downscaler https://raw.githubusercontent.com/Raffo/kube-nodes-downscaler/master/charts/
helm install --name kube-nodes-downscaler raffo-kube-nodes-downscaler/kube-nodes-downscaler
```  
