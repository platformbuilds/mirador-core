{{- define "vitess-minimal.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "vitess-minimal.etcdClientHost" -}}
{{- if .Values.etcd.clientHost -}}
{{ .Values.etcd.clientHost }}
{{- else -}}
{{ printf "%s-etcd" .Release.Name }}
{{- end -}}
{{- end -}}

{{- define "vitess-minimal.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "vitess-minimal.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
