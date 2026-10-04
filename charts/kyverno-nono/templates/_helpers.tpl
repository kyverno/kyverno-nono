{{- define "kyverno-nono.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kyverno-nono.fullname" -}}
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

{{- define "kyverno-nono.labels" -}}
{{- $labels := dict
  "app.kubernetes.io/name" (include "kyverno-nono.name" .)
  "app.kubernetes.io/instance" .Release.Name
  "app.kubernetes.io/version" .Chart.AppVersion
  "app.kubernetes.io/managed-by" .Release.Service
  "helm.sh/chart" (printf "%s-%s" .Chart.Name .Chart.Version)
-}}
{{- range $key, $value := .Values.commonLabels -}}
{{- if not (hasKey $labels $key) -}}
{{- $_ := set $labels $key $value -}}
{{- end -}}
{{- end -}}
{{- toYaml $labels -}}
{{- end -}}

{{- define "kyverno-nono.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "kyverno-nono.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- required "serviceAccount.name is required when serviceAccount.create is false" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "kyverno-nono.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}
