{{/*
Expand the name of the chart.
*/}}
{{- define "inrun.name" -}}
{{- default .Chart.Name .Values.global.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "inrun.fullname" -}}
{{- if .Values.global.fullnameOverride }}
{{- .Values.global.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.global.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Chart label — name + version.
Used for Helm chart tracking and upgrades.
*/}}
{{- define "inrun.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every resource.
Used for resource identification and grouping.
*/}}
{{- define "inrun.labels" -}}
helm.sh/chart: {{ include "inrun.chart" . }}
{{ include "inrun.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels — used in Deployment selector and Service selector.
MUST remain stable across upgrades (do not change).
*/}}
{{- define "inrun.selectorLabels" -}}
app.kubernetes.io/name: inrun
app.kubernetes.io/tag: inrun-internal
inrun.dev/deletion-protection: "true"
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Runtime image — respects tag override, falls back to appVersion.
*/}}
{{- define "inrun.runtimeImage" -}}
{{- if .Values.runtime.image.tag }}
{{- printf "%s:%s" .Values.runtime.image.repository .Values.runtime.image.tag }}
{{- else }}
{{- printf "%s:%s" .Values.runtime.image.repository .Chart.AppVersion }}
{{- end }}
{{- end }}

{{/*
Blueprint ConfigMap the runtime mounts.
*/}}
{{- define "inrun.blueprintConfigMapName" -}}
{{- if .Values.runtime.blueprint.existingConfigMap }}
{{- .Values.runtime.blueprint.existingConfigMap }}
{{- else }}
{{- printf "%s-blueprint" (include "inrun.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Gateway image — respects tag override, falls back to appVersion.
*/}}
{{- define "inrun.gatewayImage" -}}
{{- if .Values.gateway.image.tag }}
{{- printf "%s:%s" .Values.gateway.image.repository .Values.gateway.image.tag }}
{{- else }}
{{- printf "%s:%s" .Values.gateway.image.repository .Chart.AppVersion }}
{{- end }}
{{- end }}

{{/*
Blueprint ConfigMap the gateway mounts — its own, or the runtime's.
*/}}
{{- define "inrun.gatewayBlueprintConfigMapName" -}}
{{- default (include "inrun.blueprintConfigMapName" .) .Values.gateway.blueprint.existingConfigMap }}
{{- end }}

{{/*
Console image — respects tag override, falls back to appVersion.
*/}}
{{- define "inrun.consoleImage" -}}
{{- if .Values.console.image.tag }}
{{- printf "%s:%s" .Values.console.image.repository .Values.console.image.tag }}
{{- else }}
{{- printf "%s:%s" .Values.console.image.repository .Chart.AppVersion }}
{{- end }}
{{- end }}

{{/*
Runtime URLs the console watches, comma-separated. Defaults to this release's runtime.
*/}}
{{- define "inrun.consoleURLs" -}}
{{- if .Values.console.config.inrunURLs }}
{{- join "," .Values.console.config.inrunURLs }}
{{- else }}
{{- printf "http://%s-runtime:%v" (include "inrun.fullname" .) .Values.runtime.server.httpPort }}
{{- end }}
{{- end }}
