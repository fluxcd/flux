apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: helloworld
spec:
  minReadySeconds: 5
  replicas: 2
  template:
    metadata:
      labels:
        name: helloworld
    spec:
      containers:
      - name: helloworld
        image: quay.io/weaveworks/helloworld:{{ .ImageTag }}
        args:
        - -msg=Ahoy
        ports:
        - containerPort: 80
      - name: sidecar
        image: quay.io/weaveworks/sidecar:{{ .ImageTag }}
        args:
        - -addr=:8080
        ports:
        - containerPort: 8080
