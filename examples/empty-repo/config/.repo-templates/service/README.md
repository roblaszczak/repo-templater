# {{ .HumanName }}

## Running

    make run

## Libs

{{ range .Config.Repositories -}}
    {{ $repoConfig := . -}}
    {{ range .Templates -}}
        {{ if eq (.) "lib" -}}
        - {{ $repoConfig.Variables.GoPackage -}}
        {{ end -}}
    {{ end -}}
{{ end }}
