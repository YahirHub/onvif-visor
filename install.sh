#!/bin/sh
set -eu

MIN_GO_MAJOR=1
MIN_GO_MINOR=24
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TMP_DIR=""
PACKAGE_MANAGER=""

cleanup() {
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT INT TERM

log() {
  printf '%s\n' "[onvif-visor] $*" >&2
}

fail() {
  printf '%s\n' "[onvif-visor] ERROR: $*" >&2
  exit 1
}

have() {
  command -v "$1" >/dev/null 2>&1
}

retry() {
  attempts=$1
  shift
  count=1
  while :; do
    if "$@"; then
      return 0
    fi
    if [ "$count" -ge "$attempts" ]; then
      return 1
    fi
    count=$((count + 1))
    log "El comando falló; reintentando ($count/$attempts)..."
    sleep 3
  done
}

as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif have sudo; then
    sudo "$@"
  elif have doas; then
    doas "$@"
  elif have pkexec; then
    pkexec "$@"
  else
    fail "Se necesitan privilegios de administrador. Instala sudo/doas/pkexec o ejecuta este script como root."
  fi
}

detect_package_manager() {
  for pm in apk apt-get dnf yum pacman zypper; do
    if have "$pm"; then
      PACKAGE_MANAGER="$pm"
      return 0
    fi
  done
  fail "No se detectó un gestor soportado (apk, apt, dnf, yum, pacman o zypper)."
}

build_dependencies_present() {
  have gcc && have make && have go && have curl && have git && have tar && have gzip
}

install_build_dependencies() {
  if build_dependencies_present; then
    log "Toolchain base ya disponible; no se reinstalarán paquetes innecesariamente."
    return
  fi
  log "Instalando compilador, Go del sistema, curl, Git, Make y herramientas base con $PACKAGE_MANAGER..."
  case "$PACKAGE_MANAGER" in
    apk)
      retry 3 as_root apk add --no-cache build-base make go curl git ca-certificates tar gzip
      ;;
    apt-get)
      retry 3 as_root apt-get update
      retry 3 as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y build-essential make golang-go curl git ca-certificates tar gzip
      ;;
    dnf)
      retry 3 as_root dnf install -y gcc gcc-c++ make golang curl git ca-certificates tar gzip
      ;;
    yum)
      retry 3 as_root yum install -y gcc gcc-c++ make golang curl git ca-certificates tar gzip
      ;;
    pacman)
      retry 3 as_root pacman -Sy --needed --noconfirm base-devel go make curl git ca-certificates tar gzip
      ;;
    zypper)
      retry 3 as_root zypper --non-interactive install gcc gcc-c++ make go curl git ca-certificates tar gzip
      ;;
  esac
}

ensure_ffmpeg() {
  if have ffmpeg && have ffprobe; then
    log "FFmpeg detectado: $(ffmpeg -version 2>/dev/null | sed -n '1p')"
    return
  fi
  log "Instalando FFmpeg..."
  case "$PACKAGE_MANAGER" in
    apk) as_root apk add --no-cache ffmpeg ;;
    apt-get) as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y ffmpeg ;;
    dnf) as_root dnf install -y ffmpeg || fail "Tu repositorio DNF no ofrece FFmpeg completo. Habilita un repositorio que proporcione ffmpeg y vuelve a ejecutar install.sh." ;;
    yum) as_root yum install -y ffmpeg || fail "Tu repositorio YUM no ofrece FFmpeg completo. Habilita un repositorio que proporcione ffmpeg y vuelve a ejecutar install.sh." ;;
    pacman) as_root pacman -S --needed --noconfirm ffmpeg ;;
    zypper) as_root zypper --non-interactive install ffmpeg ;;
  esac
  have ffmpeg && have ffprobe || fail "FFmpeg/ffprobe no quedaron disponibles en PATH."
}

go_version_ok() {
  have go || return 1
  version=$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')
  major=${version%%.*}
  rest=${version#*.}
  minor=${rest%%.*}
  case "$major:$minor" in
    *[!0-9:]*|:*) return 1 ;;
  esac
  [ "$major" -gt "$MIN_GO_MAJOR" ] || { [ "$major" -eq "$MIN_GO_MAJOR" ] && [ "$minor" -ge "$MIN_GO_MINOR" ]; }
}

install_current_go() {
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) go_arch=amd64 ;;
    aarch64|arm64) go_arch=arm64 ;;
    *) fail "Arquitectura no soportada para descarga automática de Go: $arch" ;;
  esac

  latest=$(curl -fsSL 'https://go.dev/VERSION?m=text' | sed -n '1p')
  case "$latest" in
    go[0-9]*) ;;
    *) fail "No se pudo determinar la versión estable actual de Go." ;;
  esac

  TMP_DIR=$(mktemp -d)
  archive="$TMP_DIR/$latest.linux-$go_arch.tar.gz"
  log "Instalando $latest desde go.dev..."
  curl -fL --retry 3 --connect-timeout 15 -o "$archive" "https://go.dev/dl/$latest.linux-$go_arch.tar.gz"
  as_root rm -rf /usr/local/go
  as_root tar -C /usr/local -xzf "$archive"
  as_root ln -sf /usr/local/go/bin/go /usr/local/bin/go
  as_root ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
  hash -r 2>/dev/null || true
  cleanup
  TMP_DIR=""
}

ensure_go() {
  if go_version_ok; then
    log "Go compatible detectado: $(go version)"
    return
  fi
  log "La versión del gestor no cumple Go >= ${MIN_GO_MAJOR}.${MIN_GO_MINOR}; se instalará una versión estable oficial."
  install_current_go
  go_version_ok || fail "Go quedó instalado pero no cumple la versión mínima requerida."
}

find_onnxruntime() {
  for candidate in \
    /usr/lib/libonnxruntime.so.1 \
    /usr/local/lib/libonnxruntime.so.1 \
    /usr/lib/libonnxruntime.so \
    /usr/local/lib/libonnxruntime.so; do
    if [ -e "$candidate" ]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  for candidate in /usr/lib/libonnxruntime.so.* /usr/local/lib/libonnxruntime.so.*; do
    if [ -e "$candidate" ]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

try_native_onnxruntime() {
  case "$PACKAGE_MANAGER" in
    apk)
      as_root apk add --no-cache onnxruntime >/dev/null 2>&1 || return 1
      ;;
    apt-get)
      if have apt-cache && apt-cache show libonnxruntime-dev >/dev/null 2>&1; then
        as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y libonnxruntime-dev >/dev/null
      else
        return 1
      fi
      ;;
    dnf)
      dnf -q list onnxruntime >/dev/null 2>&1 || return 1
      as_root dnf install -y onnxruntime >/dev/null
      ;;
    yum)
      yum -q list onnxruntime >/dev/null 2>&1 || return 1
      as_root yum install -y onnxruntime >/dev/null
      ;;
    pacman)
      pacman -Si onnxruntime >/dev/null 2>&1 || return 1
      as_root pacman -S --needed --noconfirm onnxruntime >/dev/null
      ;;
    zypper)
      zypper --non-interactive search -x onnxruntime >/dev/null 2>&1 || return 1
      as_root zypper --non-interactive install onnxruntime >/dev/null
      ;;
    *) return 1 ;;
  esac
}

is_musl() {
  ldd --version 2>&1 | grep -qi musl && return 0
  for loader in /lib/ld-musl-*.so.1; do
    [ -e "$loader" ] && return 0
  done
  return 1
}

install_official_onnxruntime() {
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) ort_arch=x64 ;;
    aarch64|arm64) ort_arch=aarch64 ;;
    *) fail "Arquitectura no soportada para ONNX Runtime oficial: $arch" ;;
  esac

  latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' https://github.com/microsoft/onnxruntime/releases/latest)
  tag=${latest_url##*/}
  case "$tag" in
    v[0-9]*) ;;
    *) fail "No se pudo resolver la versión más reciente de ONNX Runtime." ;;
  esac
  version=${tag#v}

  TMP_DIR=$(mktemp -d)
  archive="$TMP_DIR/onnxruntime.tgz"
  asset="onnxruntime-linux-$ort_arch-$version.tgz"
  url="https://github.com/microsoft/onnxruntime/releases/download/$tag/$asset"
  log "Descargando ONNX Runtime $tag oficial para $ort_arch..."
  curl -fL --retry 3 --connect-timeout 15 -o "$archive" "$url"
  tar -xzf "$archive" -C "$TMP_DIR"
  extracted="$TMP_DIR/onnxruntime-linux-$ort_arch-$version"
  [ -d "$extracted/lib" ] || fail "El paquete descargado de ONNX Runtime no tiene la estructura esperada."
  as_root mkdir -p /usr/local/lib
  as_root sh -c 'cp -a "$1"/libonnxruntime.so* /usr/local/lib/' sh "$extracted/lib"
  if have ldconfig; then
    as_root ldconfig
  fi
  cleanup
  TMP_DIR=""
}

ensure_onnxruntime() {
  if path=$(find_onnxruntime); then
    printf '%s\n' "$path"
    return 0
  fi

  log "Intentando instalar ONNX Runtime desde el gestor del sistema..."
  if try_native_onnxruntime && path=$(find_onnxruntime); then
    printf '%s\n' "$path"
    return 0
  fi

  if is_musl; then
    fail "No se encontró un paquete nativo de ONNX Runtime para este sistema musl. El paquete binario oficial de GitHub usa glibc y no se instalará de forma insegura."
  fi

  install_official_onnxruntime
  path=$(find_onnxruntime) || fail "ONNX Runtime se descargó, pero libonnxruntime.so no quedó disponible."
  printf '%s\n' "$path"
}

main() {
  [ "$(uname -s)" = "Linux" ] || fail "install.sh actualmente soporta Linux."
  detect_package_manager
  install_build_dependencies
  ensure_go
  ensure_ffmpeg

  ORT_PATH=$(ensure_onnxruntime)
  log "ONNX Runtime: $ORT_PATH"

  have gcc || fail "No se encontró gcc después de instalar dependencias."
  have ffmpeg || fail "No se encontró ffmpeg después de instalar dependencias."
  have ffprobe || fail "No se encontró ffprobe; debe venir con FFmpeg."
  have make || fail "No se encontró make después de instalar dependencias."

  cd "$ROOT_DIR"
  log "Descargando módulos Go..."
  go mod download

  if [ "${ONVIF_SKIP_TESTS:-0}" != "1" ]; then
    log "Ejecutando pruebas y go vet..."
    go test ./...
    go vet ./...
  fi

  log "Compilando ONVIF Visor con CGO y NanoDet..."
  make build

  log "Instalando/actualizando el servicio..."
  "$ROOT_DIR/dist/onvif-visor" --install "$@" --source-dir "$ROOT_DIR" -onnxruntime "$ORT_PATH"

  log "Instalación terminada."
  "$ROOT_DIR/dist/onvif-visor" --service status || true
}

main "$@"
