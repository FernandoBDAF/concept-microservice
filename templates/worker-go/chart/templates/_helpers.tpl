{{- define "worker-go.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "worker-go.labels" -}}
app: {{ include "worker-go.name" . }}
app.kubernetes.io/name: {{ include "worker-go.name" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
