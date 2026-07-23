{{/* Chart name (overridable). */}}
{{- define "trqsh.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Fully qualified app name. */}}
{{- define "trqsh.fullname" -}}
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

{{- define "trqsh.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Common labels. */}}
{{- define "trqsh.labels" -}}
helm.sh/chart: {{ include "trqsh.chart" . }}
app.kubernetes.io/name: {{ include "trqsh.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: trqsh
{{- end -}}

{{/* Selector labels for a given component (pass a dict: root . and component name). */}}
{{- define "trqsh.selectorLabels" -}}
app.kubernetes.io/name: {{ include "trqsh.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{/* Image reference for a component repository. Usage:
     include "trqsh.image" (dict "root" . "repo" .Values.api.repository) */}}
{{- define "trqsh.image" -}}
{{- $tag := .root.Values.image.tag | default .root.Chart.AppVersion -}}
{{- printf "%s/%s:%s" .root.Values.image.registry .repo $tag -}}
{{- end -}}

{{/* Name of the Secret holding shared env (existing or chart-created). */}}
{{- define "trqsh.secretName" -}}
{{- if .Values.secrets.existingSecret -}}
{{- .Values.secrets.existingSecret -}}
{{- else -}}
{{- printf "%s-secrets" (include "trqsh.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "trqsh.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "trqsh.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}
