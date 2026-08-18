{{/*
扩展 chart 名称（不含 version）
*/}}
{{- define "fenghuo.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
完整 release 名称
*/}}
{{- define "fenghuo.fullname" -}}
{{- if contains .Chart.Name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "fenghuo.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
通用标签
*/}}
{{- define "fenghuo.labels" -}}
helm.sh/chart: {{ include "fenghuo.chart" . }}
{{ include "fenghuo.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end }}

{{- define "fenghuo.selectorLabels" -}}
app.kubernetes.io/name: {{ include "fenghuo.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
应用 Secret 名称：优先使用用户提供的 existingSecret，
否则使用 chart 生成的 <fullname>-secret
*/}}
{{- define "fenghuo.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- include "fenghuo.fullname" . }}-secret
{{- end }}
{{- end }}

{{/*
URL 路径前缀（子路径部署）：从 rootUrl 提取并去掉尾斜杠，
如 "https://example.com/fenghuo/" -> "/fenghuo"，根路径部署时为""。
探针等容器内访问路径都要加此前缀（后端所有路由挂在 base path 下）
*/}}
{{- define "fenghuo.basePath" -}}
{{- get (urlParse .Values.rootUrl) "path" | default "" | trimSuffix "/" -}}
{{- end }}
