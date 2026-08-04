

# bz

[![CI](https://github.com/albertocavalcante/bz/actions/workflows/ci.yml/badge.svg)](https://github.com/albertocavalcante/bz/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/albertocavalcante/bz)](https://goreportcard.com/report/github.com/albertocavalcante/bz)
[![Go Reference](https://pkg.go.dev/badge/github.com/albertocavalcante/bz.svg)](https://pkg.go.dev/github.com/albertocavalcante/bz)
[![License](https://img.shields.io/badge/license-MIT%2FApache--2.0-blue)](LICENSE-MIT)
[![Release](https://img.shields.io/github/v/release/albertocavalcante/bz)](https://github.com/albertocavalcante/bz/releases/latest)

Una CLI para Bzlmod, el sistema de módulos de Bazel.

Gestiona las dependencias de MODULE.bazel, consulta el Registro Central de Bazel y optimiza tu flujo de trabajo de módulos de Bazel.

## Características

- **Gestión de módulos** - Agrega, elimina, lista y actualiza dependencias en MODULE.bazel
- **Análisis de dependencias** - Visualiza grafos de dependencias, analiza por qué se incluyen ciertos módulos
- **Escaneo de seguridad** - Audita dependencias en busca de vulnerabilidades, verifica el cumplimiento de licencias
- **Generación de SBOM** - Genera listas de materiales de software en formato SPDX o CycloneDX
- **Soporte para entornos aislados (air-gap)** - Modo completamente sin conexión con caché local para entornos desconectados
- **Sincronización de registros** - Replica módulos del BCR a registros internos con configuración en Starlark

## Instalación

```bash
go install github.com/albertocavalcante/bz@latest
```

O descarga un binario desde [Lanzamientos](https://github.com/albertocavalcante/bz/releases).

## Inicio Rápido

```bash
# Inicializa un nuevo módulo
bz init --name=my_project

# Agrega dependencias
bz mod add rules_go@0.50.1 rules_python@0.35.0

# Comprueba si hay actualizaciones
bz mod outdated

# Actualiza todas las dependencias
bz mod update

# Visualiza el grafo de dependencias
bz mod graph

# Escanea en busca de vulnerabilidades
bz audit
```

## Comandos

| Comando               | Descripción                                   |
| --------------------- | --------------------------------------------- |
| `bz init`             | Inicializa un nuevo archivo MODULE.bazel      |
| `bz mod add`          | Agrega dependencias                           |
| `bz mod rm`           | Elimina dependencias                          |
| `bz mod list`         | Lista todas las dependencias                  |
| `bz mod info`         | Muestra información del módulo desde el registro |
| `bz mod update`       | Actualiza dependencias a las últimas versiones|
| `bz mod outdated`     | Comprueba si hay versiones más nuevas         |
| `bz mod search`       | Busca módulos en el registro                  |
| `bz mod sync`         | Sincroniza módulos entre registros            |
| `bz mod graph`        | Muestra el grafo de dependencias              |
| `bz mod stats`        | Muestra estadísticas de dependencias          |
| `bz mod why`          | Explica por qué se incluye un módulo          |
| `bz mod licenses`     | Muestra información de licencias              |
| `bz audit`            | Escanea vulnerabilidades (vía OSV)            |
| `bz sbom`             | Genera SBOM (SPDX/CycloneDX)                  |
| `bz cache download`   | Descarga módulos al caché local               |
| `bz cache verify`     | Verifica la integridad del caché              |
| `bz cache clear`      | Limpia el caché local                         |
| `bz doctor`           | Verifica la configuración de Bazel/bzlmod     |
| `bz registry ping`    | Prueba la conectividad con el registro        |
| `bz completion`       | Genera complementos para shell                |

Ejecuta `bz <command> --help` para obtener detalles de uso, o consulta la [Referencia de CLI](https://albertocavalcante.github.io/bz/cli/).

## Configuración

bz lee la configuración desde archivos TOML y variables de entorno:

| Ubicación                      | Propósito              |
| ------------------------------ | ---------------------- |
| `~/.config/bz/config.toml`     | Configuración de usuario |
| `.bz.toml`                     | Configuración del proyecto |

```toml
[network]
mode = "prefer-offline"
registry = "https://bcr.bazel.build"
timeout = "30s"

[cache]
dir = "~/.cache/bz"
ttl = "24h"
```

Consulta la [Guía de Configuración](https://albertocavalcante.github.io/bz/configuration/) para conocer las variables de entorno, reglas de precedencia y la configuración basada en Starlark.

## Documentación

- [Documentación completa](https://albertocavalcante.github.io/bz/)
- [Primeros pasos](https://albertocavalcante.github.io/bz/getting-started/)
- [Referencia de CLI](https://albertocavalcante.github.io/bz/cli/)
- [Configuración](https://albertocavalcante.github.io/bz/configuration/)
- [Configuración en entornos aislados](https://albertocavalcante.github.io/bz/guides/air-gapped/)
- [Replicación de registros](https://albertocavalcante.github.io/bz/guides/mirror-bcr/)

## Contribuir

Consulta [CONTRIBUTING.md](.github/CONTRIBUTING.md) para comenzar.

## Licencia

Licenciado bajo la [Licencia Apache, versión 2.0](LICENSE-APACHE) o la [licencia MIT](LICENSE-MIT) a tu elección.
