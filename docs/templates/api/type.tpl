{{- define "type" -}}
{{- $type := . -}}
{{- if markdownShouldRenderType $type -}}

#### {{ $type.Name }}

{{ if $type.IsAlias }}_Underlying type:_ _{{ markdownRenderTypeLink $type.UnderlyingType }}_{{ end }}

{{ $type.Doc }}

{{ if $type.Validation -}}
_Validation:_
{{- range $type.Validation }}
- {{ . }}
{{- end }}
{{- end }}

{{- if $type.References -}}
Appears in: {{ range $i, $ref := $type.SortedReferences }}{{ if $i }}, {{ end }}{{ markdownRenderTypeLink $ref }}{{- end }}
{{- end }}

{{ if $type.Members -}}
| Field | Description |
| --- | --- |
{{ if $type.GVK -}}
| apiVersion<br/>_string_ | (Required)<br/>`{{ $type.GVK.Group }}/{{ $type.GVK.Version }}` |
| kind<br/>_string_ | (Required)<br/>`{{ $type.GVK.Kind }}` |
{{ end -}}
{{- $members := default dict -}}
{{- range $member := $type.Members -}}
{{- $_ := set $members $member.Name $member }}
{{- end -}}
{{- $memberKeys := (keys $members | sortAlpha) -}}
{{ range $memberKeys -}}
{{- $member := index $members . -}}
{{- $id := lower (printf "%s-%s" $type.Name $member.Name) -}}
| {{ $member.Name }}<a href="#{{ $id }}" id="{{ $id }}">#</a><br/>_{{ markdownRenderType $member.Type }}_ | {{ if $member.Markers.optional }}_(Optional)_<br/>{{else}}_(Required)_<br/>{{ end }}{{ template "type_members" $member }} |
{{ end -}}

{{- end -}}
{{- end -}}
{{- end -}}
