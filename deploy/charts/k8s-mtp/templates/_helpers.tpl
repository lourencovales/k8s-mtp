{{- define "k8s-mtp.image" -}}
{{ .Values.image.registry }}/{{ .name }}:{{ .Values.image.tag }}
{{- end }}
