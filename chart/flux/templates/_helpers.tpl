{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "flux.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "flux.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "flux.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "flux.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "flux.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Create the name of the cluster role to use
*/}}
{{- define "flux.clusterRoleName" -}}
{{- if .Values.clusterRole.create -}}
    {{ default (include "flux.fullname" .) .Values.clusterRole.name }}
{{- else -}}
    {{ default "default" .Values.clusterRole.name }}
{{- end -}}
{{- end -}}

{{/*
Create a custom repositories.yaml for Helm
*/}}
{{- define "flux.customRepositories" -}}
apiVersion: v1
generated: 0001-01-01T00:00:00Z
repositories:
{{- range .Values.helmOperator.configureRepositories.repositories }}
- name: {{ required "Please specify a name for the Helm repo" .name }}
  url: {{ required "Please specify a URL for the Helm repo" .url }}
  cache: /var/fluxd/helm/repository/cache/{{ .name }}-index.yaml
  caFile: ""
  certFile: ""
  keyFile: ""
  password: "{{ .password | default "" }}"
  username: "{{ .username | default "" }}"
{{- end }}
{{- end -}}

{{/*
Create the name of the Git config Secret.
*/}}
{{- define "git.config.secretName" -}}
{{- if .Values.git.config.enabled }}
    {{- if .Values.git.config.secretName -}}
        {{ default "default" .Values.git.config.secretName }}
    {{- else -}}
        {{ default (printf "%s-git-config" (include "flux.fullname" .)) }}
{{- end -}}
{{- end }}
{{- end }}
