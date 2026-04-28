{{- define "tyk-sre-assignment.name" -}}
{{- .Chart.Name }}
{{- end }}

{{- define "tyk-sre-assignment.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "tyk-sre-assignment.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/name: {{ include "tyk-sre-assignment.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "tyk-sre-assignment.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tyk-sre-assignment.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}