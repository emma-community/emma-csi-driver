{{/*
Expand the name of the chart.
*/}}
{{- define "emma-csi-driver.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "emma-csi-driver.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "emma-csi-driver.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "emma-csi-driver.labels" -}}
helm.sh/chart: {{ include "emma-csi-driver.chart" . }}
{{ include "emma-csi-driver.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "emma-csi-driver.selectorLabels" -}}
app.kubernetes.io/name: {{ include "emma-csi-driver.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Controller labels
*/}}
{{- define "emma-csi-driver.controller.labels" -}}
{{ include "emma-csi-driver.labels" . }}
app: emma-csi-controller
{{- end }}

{{/*
Node labels
*/}}
{{- define "emma-csi-driver.node.labels" -}}
{{ include "emma-csi-driver.labels" . }}
app: emma-csi-node
{{- end }}

{{/*
Controller service account name
*/}}
{{- define "emma-csi-driver.controller.serviceAccountName" -}}
{{- if .Values.controller.serviceAccount.create }}
{{- default "emma-csi-controller-sa" .Values.controller.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.controller.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Node service account name
*/}}
{{- define "emma-csi-driver.node.serviceAccountName" -}}
{{- if .Values.node.serviceAccount.create }}
{{- default "emma-csi-node-sa" .Values.node.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.node.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Secret name for Emma credentials
*/}}
{{- define "emma-csi-driver.secretName" -}}
{{- if .Values.emma.credentials.existingSecret }}
{{- .Values.emma.credentials.existingSecret }}
{{- else }}
{{- include "emma-csi-driver.fullname" . }}-credentials
{{- end }}
{{- end }}

{{/*
Namespace
*/}}
{{- define "emma-csi-driver.namespace" -}}
{{- if .Values.namespaceOverride }}
{{- .Values.namespaceOverride }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}
