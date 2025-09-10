{{- define "vitess-minimal.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Render an image string with optional global registry prefix */}}
{{- define "vitess-minimal.renderImage" -}}
{{- $img := .image -}}
{{- $vals := .Values -}}
{{- $reg := "" -}}
{{- if $vals.global }}
  {{- $reg = $vals.global.imageRegistry | default "" -}}
{{- end -}}
{{- if $reg }}{{ printf "%s/%s" $reg $img }}{{ else }}{{ $img }}{{ end -}}
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
