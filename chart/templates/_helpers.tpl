{{- define "mirador-core.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mirador-core.fullname" -}}
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

{{- define "mirador-core.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "mirador-core.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
  Valkey service helpers. Bitnami Valkey exposes services named
  "<release>-valkey-primary" and "<release>-valkey-headless" by default.
  Allow override via .Values.valkey.serviceName / headlessServiceName.
*/}}
{{- define "mirador-core.valkeyServiceHost" -}}
{{- if .Values.valkey.serviceName -}}
{{ .Values.valkey.serviceName }}
{{- else -}}
{{ printf "%s-valkey-primary" .Release.Name }}
{{- end -}}
{{- end -}}

{{- define "mirador-core.valkeyHeadlessHost" -}}
{{- if .Values.valkey.headlessServiceName -}}
{{ .Values.valkey.headlessServiceName }}
{{- else -}}
{{ printf "%s-valkey-headless" .Release.Name }}
{{- end -}}

{{/* Vitess vtgate host helper for the subchart (if enabled) */}}
{{- define "mirador-core.vitessVtgateHost" -}}
{{- /* Subchart fullname will be <release>-vitess-minimal */ -}}
{{ printf "%s-vitess-minimal-vtgate" .Release.Name }}
{{- end -}}
{{- end -}}
